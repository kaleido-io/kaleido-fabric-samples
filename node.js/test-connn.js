var http = require("https");

var options = {
  host: 'hun-peer.api-dev.healthutilitynetwork.org',
  port: 443,
  method: 'GET'
};

var req = http.request(options, function(res) {
  console.log('STATUS: ' + res.statusCode);
  console.log('HEADERS: ' + JSON.stringify(res.headers));
});

req.on('error', function(e) {
  console.log('problem with request: ' + e.message);
});

req.end();

