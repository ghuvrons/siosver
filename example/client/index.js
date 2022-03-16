import express from "express"
import http from "http"
import path from 'path';
import { fileURLToPath } from 'url';


const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

const app = express();
const server = http.createServer(app);

app.get('/', (req, res) => {
  res.sendFile(__dirname + '/index.html');
});
app.get('/tester.js', (req, res) => {
  res.sendFile(__dirname + '/tester.js');
});

console.log("start")
server.listen(3000, () => {
  console.log('listening on http://localhost:3000');
});
