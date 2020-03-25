import { Hterm } from "./hterm";
import { Xterm } from "./xterm";
import { Terminal, WebTTY, protocols } from "./webtty";
import { ConnectionFactory } from "./websocket";

// @TODO remove these
declare var gotty_auth_token: string;
declare var gotty_term: string;
declare var ws_uri: string;
declare var t_id: string;
declare var s_id: string;
declare var c_id: string;
declare var md5: string;

const elem = document.getElementById("terminal");

if (elem !== null) {
  var term: Terminal;
  if (gotty_term == "hterm") {
    term = new Hterm(elem);
  } else {
    term = new Xterm(elem);
  }
  //const url = ws_uri
  const httpsEnabled = window.location.protocol == "https:";
  const url = ws_uri
    ? ws_uri
    : (httpsEnabled ? "wss://" : "ws://") +
      window.location.host +
      window.location.pathname +
      "ws";
  //const args = window.location.search;
  const args = {
    T_id: t_id,
    S_id: s_id,
    C_id: c_id,
    Md5: md5
  };
  const factory = new ConnectionFactory(url, protocols);
  const wt = new WebTTY(term, factory, args, gotty_auth_token);
  const closer = wt.open();

  window.addEventListener("unload", () => {
    closer();
    term.close();
  });
}
