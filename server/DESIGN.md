# Status Board Server Design

The status board is a set of three web applications that display the running
status of the exercise in various ways:
- an overview application (at /)
- a log viewer application (at /log)
- a station monitor application (at /station/«CALLSIGN»)
The server supports multiple simultaneous instances of the web applications.  In
addition, the server allows GET requests for /message/«LMI».pdf, which generates
(if needed) and serve the PDF of a message.

Each of the three applications returns a static, self-contained HTML document.
Scripts in that document establish a websocket connection to the server (/ws,
/ws/log, /ws/station/«CALLSIGN», respectively) which is used to retrieve and
update the dynamic data.  The station monitor app can also send requests to the
server over that websocket connection to manually trigger exercise events.

The server automatically closes the websockets for the overview and station
monitor apps when the exercise definition changes.  This is their cue to reload
all pertinent data.
