var fs = require('fs');
var express = require('express')
var exec = require('child_process').exec;
var app = express()

var portDiff = parseInt(process.argv[3]) || 0;

var frontPort = process.argv[2] - 0;
var appPort = frontPort + portDiff;

if (!(appPort >= 2000 && appPort <= 30000)) {
  throw new Error("Bad port: " + process.argv[2]);
}

if (portDiff) {
  startProxy(frontPort, appPort);
}

app.get('/', function (req, res) {
  console.log('/');
  res.send('Hello World!')
});

app.get('/status', function (req, res) {
  console.log('/status');
  res.send("I'm ok!");
});

app.get('/status500', function (req, res) {
  console.log('/status500');
  res.status(500).send("I'm not OK!");
});

app.get('/statusTimeout', function (req, res) {
  console.log('/statusTimeout - not going to send response');
});

app.get('/file', function (req, res) {
  try {
    var data = fs.readFileSync('data/file');
    res.send(data);
  } catch (e) {
    res.send('<no file>');
  }
});

var server = app.listen(appPort, function () {

  var host = server.address().address
  var port = server.address().port

  console.log('Example app listening at http://%s:%s', host, port)

});

function startProxy(frontPort, appPort) {
  var cmd = './start-haproxy.sh ' + frontPort + ' '+ appPort;
  console.log("run haproxy on %s", frontPort);
  exec(cmd, function(err, out, code) {
    if (err instanceof Error) {
      throw err;
    }
    process.exit(code);
  });
}
