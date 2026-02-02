const http = require("http");
const fs = require("fs");
const path = require("path");

http
  .createServer((req, res) => {
    console.log("starting...");
    const file = req.url === "/" ? "index.html" : req.url.slice(1);
    const p = path.join(__dirname, file);
    fs.readFile(p, (err, data) => {
      if (err) {
        res.writeHead(404);
        return res.end("Not found");
      }
      res.writeHead(200, { "Content-Type": "text/html; charset=utf-8" });
      res.end(data);
    });
  })
  .listen(8080, () => console.log("http://localhost:8080"));
