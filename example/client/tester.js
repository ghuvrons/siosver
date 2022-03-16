const testerList = [
  {name: "connecting", isSuccess: false},
  {name: "authentication", isSuccess: false},
  {name: "emit_standart", isSuccess: false},
  {name: "emit_json", isSuccess: false},
  {name: "emit_binary", isSuccess: false},
];

const TEST_CONNECT = 0;

var socketIo = io("ws://localhost:8000", { transports: ["websocket"] });
socketIo.on("connect", () => {
  console.log("connected id:", socketIo.id);
  testerList[TEST_CONNECT].isSuccess = true;
  run();
});

socketIo.on("test_bin", (msg) => {
  console.log("<", msg);
  let view = new Int8Array(msg.data);
  console.log(view)
});

async function run() {
  // // emit standart
  // socketIo.emit("send-number", 90);

  // // emit json
  // let jsonData = {name: "janoko"};
  // socketIo.emit("send-json", jsonData);

  // // emit buffer
  // let buffer = new ArrayBuffer(8);
  // socketIo.emit("send-buffer", buffer);
  return;
}
