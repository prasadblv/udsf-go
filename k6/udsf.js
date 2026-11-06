import http from 'k6/http';
import { check, sleep } from 'k6';
import exec from 'k6/execution';

const VUS = 1000;
const POOL = 10; 

export const options = {
  vus: VUS,
  teardownTimeout: '120s',
  duration: '120s',
  
  insecureSkipTLSVerify: true,
  
};

const BASE_URL = __ENV.BASE_URL || 'https://127.0.0.1:9443/nudsf-dr/v1';
const REALM = 'Realm01';
const STORAGE = 'Storage01';

export default function () {
  // Unique record and payload per VU and iteration.
  const vu = exec.vu.idInTest;
  const iter = exec.vu.iterationInScenario;
  const rec = `${BASE_URL}/${REALM}/${STORAGE}/records/Record-vu${vu}/blocks`;
  const blockUrl = `${rec}/Block-${iter % POOL}`;

  const payload = JSON.stringify({
    name: `vu-${vu}`,
    value: vu * 1000 + iter,
    ts: Date.now(),
  });
  const params = { headers: { 'Content-Type': 'application/json' } };

  const put = http.put(blockUrl, payload, params);
  check(put, {
    'PUT is HTTP/2': (r) => r.proto === 'HTTP/2.0',
    'PUT 201/204': (r) => r.status === 201 || r.status === 204,
  });

  const list = http.get(rec);
  check(list, {
    'GET is HTTP/2': (r) => r.proto === 'HTTP/2.0',
    'GET 200': (r) => r.status === 200,
  });

  sleep(1);
}

// Cleanup: delete every block any VU may have written.
// 404 is expected for blocks that were never created.
export function teardown() {
  const reqs = [];
  for (let vu = 1; vu <= VUS; vu++) {
    for (let b = 0; b < POOL; b++) {
      reqs.push(['DELETE', `${BASE_URL}/${REALM}/${STORAGE}/records/Record-vu${vu}/blocks/Block-${b}`, null, { responseCallback: http.expectedStatuses(200, 204, 404) }]);
    }
  }
  const CHUNK = 200;
  for (let i = 0; i < reqs.length; i += CHUNK) {
    http.batch(reqs.slice(i, i + CHUNK));
  }
}
