package configs

import "github.com/spf13/pflag"

type WebSocketConfig struct {
	WebsocketSSL      bool
	WebsocketCertFile string
	WebsocketKeyFile  string
	WebsocketAddr     string
}

func AddWebSocketFlags(fs *pflag.FlagSet, wsc *WebSocketConfig) {
	fs.BoolVar(&wsc.WebsocketSSL, "ws-ssl-enable", false, "whether to enable websocket  SSL")
	fs.StringVar(&wsc.WebsocketCertFile, "ws-ssl-certfile", "/etc/ssl/goodrain.com/goodrain.com.crt", "websocket and fileserver ssl cert file")
	fs.StringVar(&wsc.WebsocketKeyFile, "ws-ssl-keyfile", "/etc/ssl/goodrain.com/goodrain.com.key", "websocket and fileserver ssl key file")
	fs.StringVar(&wsc.WebsocketAddr, "ws-addr", "0.0.0.0:6060", "the websocket server listen address")
}
