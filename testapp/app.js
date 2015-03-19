var fs = require('fs');
var express = require('express')
var app = express()

var port = process.argv[2] - 0;

if (!(port >= 2000 && port <= 30000)) {
  throw new Error("Bad port: " + process.argv[2]);
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

var server = app.listen(port, function () {

  var host = server.address().address
  var port = server.address().port

  console.log('Example app listening at http://%s:%s', host, port)

});
