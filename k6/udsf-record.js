import http from 'k6/http';
import { check, sleep } from 'k6';
import exec from 'k6/execution';

// Load test for the Record CRUD API (implemented operations only):
//   PUT    /records/{id}        multipart/form-data: meta + one block
//   GET    /records/{id}
//   GET    /records/{id}/meta
//   DELETE /records/{id}        (every DELETE_EVERY-th iteration, then the record is recreated)
// Override with env: VUS, DURATION, BASE_URL, POOL, DELETE_EVERY, SLEEP.
//   k6 run k6/udsf-record.js
//   VUS=50 DURATION=30s k6 run k6/udsf-record.js

const VUS = parseInt(__ENV.VUS || '1000', 10);
const POOL = parseInt(__ENV.POOL || '10', 10);                // distinct block ids per record
const DELETE_EVERY = parseInt(__ENV.DELETE_EVERY || '10', 10); // 0 disables in-test deletes
const SLEEP = parseFloat(__ENV.SLEEP || '1');

export const options = {
  vus: VUS,
  duration: __ENV.DURATION || '120s',
  teardownTimeout: '120s',
  insecureSkipTLSVerify: true,
  thresholds: {
    checks: ['rate>0.99'],
    http_req_failed: ['rate<0.01'],
    http_req_duration: ['p(95)<500'],
    // per-operation breakdown (also makes k6 print each operation's latency)
    'http_req_duration{op:put_record}': ['p(95)<500'],
    'http_req_duration{op:get_record}': ['p(95)<500'],
    'http_req_duration{op:get_meta}': ['p(95)<500'],
    'http_req_duration{op:delete_record}': ['p(95)<500'],
  },
};

const BASE_URL = __ENV.BASE_URL || 'https://127.0.0.1:9443/nudsf-dr/v1';
const REALM = 'Realm01';
const STORAGE = 'Storage01';
const recordUrl = (vu) => `${BASE_URL}/${REALM}/${STORAGE}/records/Record-vu${vu}`;

const is2 = (r) => r.proto === 'HTTP/2.0';

export default function () {
  // One record per VU; payloads are unique per iteration.
  const vu = exec.vu.idInTest;
  const iter = exec.vu.iterationInScenario;
  const rec = recordUrl(vu);

  // PUT: creates the record on the first iteration (201), updates afterwards (204).
  // The block part's filename is used as the block id.
  const body = {
    meta: JSON.stringify({
      callbackReference: `http://cb.example/vu${vu}`,
      tags: { vu: [`vu-${vu}`], iter: [`${iter}`] },
    }),
    blocks: http.file(
      JSON.stringify({ name: `vu-${vu}`, value: vu * 1000 + iter, ts: Date.now() }),
      `Block-${iter % POOL}`,
      'application/json',
    ),
  };
  const put = http.put(rec, body, { tags: { op: 'put_record' } });
  check(put, {
    'PUT is HTTP/2': is2,
    'PUT 201/204': (r) => r.status === 201 || r.status === 204,
  });

  const get = http.get(rec, { tags: { op: 'get_record' } });
  check(get, {
    'GET is HTTP/2': is2,
    'GET 200': (r) => r.status === 200,
    'GET lists the block just written': (r) => r.status === 200 && r.body.includes(`Block-${iter % POOL}`),
  });

  const meta = http.get(`${rec}/meta`, { tags: { op: 'get_meta' } });
  check(meta, {
    'GET meta is HTTP/2': is2,
    'GET meta 200': (r) => r.status === 200,
  });

  if (DELETE_EVERY > 0 && (iter + 1) % DELETE_EVERY === 0) {
    const del = http.del(rec, null, { tags: { op: 'delete_record' } });
    check(del, {
      'DELETE is HTTP/2': is2,
      'DELETE 204': (r) => r.status === 204,
    });
  }

  sleep(SLEEP);
}

// Cleanup: delete every record any VU may have left behind (this also removes its blocks).
// 404 is expected for records that were never created or were already deleted.
export function teardown() {
  const reqs = [];
  for (let vu = 1; vu <= VUS; vu++) {
    reqs.push(['DELETE', recordUrl(vu), null, { responseCallback: http.expectedStatuses(200, 204, 404) }]);
  }
  const CHUNK = 200;
  for (let i = 0; i < reqs.length; i += CHUNK) {
    http.batch(reqs.slice(i, i + CHUNK));
  }
}
