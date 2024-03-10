
// type MessageType int
// The message types are defined in RFC 6455, section 11.8.

const MessageType = {

    // Not in RFC 6455
    ClientConnected: -1,

    // Not in RFC 6455
    ClientDisconnected: -2,

    // TextMessage denotes a text data message. The text message payload is
    // interpreted as UTF-8 encoded text data.
    TextMessage: 1,

    // BinaryMessage denotes a binary data message.
    BinaryMessage: 2,

    // CloseMessage denotes a close control message. The optional message
    // payload contains a numeric code and text. Use the FormatCloseMessage
    // function to format a close message payload.
    CloseMessage: 8,

    // PingMessage denotes a ping control message. The optional message payload
    // is UTF-8 encoded text.
    PingMessage: 9,

    // PongMessage denotes a pong control message. The optional message payload
    // is UTF-8 encoded text.
    PongMessage: 10,
}


function formatDate(date) {
    const h = "0" + date.getHours();
    const m = "0" + date.getMinutes();
    return `${h.slice(-2)}:${m.slice(-2)}`;
}

window.onload = function() {
    var conn;
    var msg = document.getElementById("msgin");
    var chat = document.getElementById("chat");

    function appendLog(item) {
        var doScroll = chat.scrollTop > chat.scrollHeight - chat.clientHeight - 1;
        chat.appendChild(item);
        if (doScroll) {
            chat.scrollTop = chat.scrollHeight - chat.clientHeight;
        }
    }

    document.getElementById("form").onsubmit = function() {
        if (!conn) return false;
        if (!msg.value) return false;
        conn.send(msg.value);
        var msgDiv = document.createElement("div");
        msgDiv.innerHTML = `<div class="msg sent"> <small class="msg sent-user-timestamp">${formatDate(new Date())}</small><br>  ${msg.value} </div>`;
        appendLog(msgDiv);
        msg.value = "";
        return false;
    };

    if (window["WebSocket"]) {
        let user_path = window.location.pathname
        let proto = window.location.protocol === "https:" ? "wss://" : "ws://";
        conn = new WebSocket(proto + document.location.host + "/ws" + user_path);
        conn.onclose = function(evt) {
            var item = document.createElement("div");
            item.innerHTML = "<b>Connection lost with server.</b>";
            item.setAttribute("class", "msg");
            appendLog(item);
        };
        conn.onmessage = function(evt) {
            var msg_data = evt.data;
            var obj = JSON.parse(msg_data)
            var msgDiv = document.createElement("div");
            var user = obj.from;
            switch (obj.type) {
                case MessageType.ClientDisconnected:
                    msgDiv.innerHTML = `<div class=""> <small class="msg rcvd-user-timestamp"> ${formatDate(new Date())} - @${user} left </small><br> </div>`;
                    break;
                case MessageType.ClientConnected:
                    msgDiv.innerHTML = `<div class=""> <small class="msg rcvd-user-timestamp"> ${formatDate(new Date())} - @${user} joined</small><br> </div>`;
                    break;
                case MessageType.TextMessage:
                    msgDiv.innerHTML = `<div class="msg rcvd"> <small class="msg rcvd-user-timestamp"> @${user} - ${formatDate(new Date())} </small><br>  ${obj.data} </div>`;
                    break;
            }
            appendLog(msgDiv);
        };
    } else {
        var item = document.createElement("div");
        item.innerHTML = "<b>Your browser does not support WebSockets.</b>";
        appendLog(item);
    }
};
