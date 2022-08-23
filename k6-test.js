import grpc from 'k6/net/grpc';
import { check, sleep } from 'k6';

const client = new grpc.Client();
client.load(['api/proto'], 'broker.proto');


export default () => {
  client.connect('localhost:8080', {
     plaintext: true
  });
  const data = {subject:"testing", body: "", expirationSeconds:60};
  const response = client.invoke('broker.Broker/Publish', data);

  check(response, {
    'status is OK': (r) => r && r.status === grpc.StatusOK,
  });

  console.log(JSON.stringify(response.message));

  client.close();
  // sleep(1);
};