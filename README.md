# Minimum Reproducible Code

## Safari Mac/iOS Bug

We are seeing examples of streams not being able to be display on Safari on Mac OS or iOS devices.

Run the WebRTC Server:

```
cd server/
go run cmd/main.go
```

Open the client

```
cd client/
open index.html
```

Accept any browser dialogs for access to Audio/Video devices.

You should see side-by-side webcam streams (left is the local stream, right is the returned stream from the server)

On Chrome, this appears to work fine
On Safari, we never see the right-hand stream