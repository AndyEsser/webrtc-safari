function createConnection() {
    let conn = new RTCPeerConnection({
        iceServers: [
            {
                "urls": "stun:mercury.haia.live",
                "username": "haia",
                "credential": "haia",
            }
        ]
    });

    conn.onconnectionstatechange = ((e) => {
        console.log("onconnectionstatechange", e);
    });

    conn.ondatachannel = ((e) => {
        console.log("ondatachannel", e);
    });

    conn.candidates = [];
    conn.onicecandidate = ((e) => {
        console.log("onicecandidate", e.candidate);

        if (conn.id) {
            let xhr2 = new XMLHttpRequest();

            xhr2.onreadystatechange  = () => {
                if (xhr2.readyState !== 4) {
                    return;
                }

                if (xhr2.status === 200) {
                    // candidate updated
                } else {
                    console.error("error sending candidate", xhr2.status, xhr2.responseText)
                }
            };
            xhr2.open('POST', 'http://localhost:8080/' + conn.id + '/candidate', true);
            xhr2.send(JSON.stringify(e.candidate));
        } else {
            conn.candidates.push(e.candidate);
        }
    });

    conn.onicecandidateerror = ((e) => {
        console.log("onicecandidateerror", e);
    });

    conn.oniceconnectionstatechange = ((e) => {
        console.log("oniceconnectionstatechange", e);
    });

    conn.onicegatheringstatechange = ((e) => {
        console.log("onicegatheringstatechange", e);
    });

    conn.onnegotiationneeded = ((e) => {
        console.log("onnegotiationneeded", e);
        // if (conn.localDescription !== null) {
        //     startConnection(conn);
        // }
    });

    conn.onsignalingstatechange = ((e) => {
        console.log("onsignalingstatechange", e);
    });

    conn.ontrack = ((e) => {
        console.log("ontrack", e);

        if (e.streams.length > 0) {
            console.log("Setting stream", e.streams[0]);
            remote.srcObject = e.streams[0];
            remote.play();
        }
    });

    return conn;
}

let rx = null;
let remote = null;

function startConnection(tx) {
    console.log("Starting connection");
    tx.createOffer()
        .then((offer) => {
            tx.setLocalDescription(offer)
                .then(() => {
                    console.log("created offer", offer);
                    let xhr = new XMLHttpRequest();

                    xhr.onreadystatechange = () => {
                        if (xhr.readyState !== 4) {
                            return;
                        }

                        if (xhr.status === 200) {
                            let answer = JSON.parse(xhr.responseText);
                            console.log("received answer", answer);

                            tx.setRemoteDescription(answer.answer)
                                .then(() => {
                                    tx.id = answer.id;
                                    console.log("exchange complete, sending cached candidates");

                                    for (let c = 0; c < tx.candidates.length; c++) {
                                        let xhr2 = new XMLHttpRequest();

                                        xhr2.onreadystatechange  = () => {
                                            if (xhr2.readyState !== 4) {
                                                return;
                                            }

                                            if (xhr2.status === 200) {
                                                // candidate updated
                                            } else {
                                                console.error("error sending candidate", xhr2.status, xhr2.responseText)
                                            }
                                        };
                                        xhr2.open('POST', 'http://localhost:8080/' + tx.id + '/candidate', true);
                                        xhr2.send(JSON.stringify(tx.candidates[c]));
                                    }

                                    let candidateUpdateInterval = setInterval(() => {
                                        let xhr3 = new XMLHttpRequest();

                                        xhr3.onreadystatechange = () => {
                                            if (xhr3.readyState !== 4) {
                                                return;
                                            }

                                            if (xhr3.status === 200) {
                                                let candidates = JSON.parse(xhr3.responseText);

                                                if (candidates && candidates.length > 0) {
                                                    if (candidates[candidates.length-1] === null) {
                                                        // Apply Candidates
                                                        clearInterval(candidateUpdateInterval);

                                                        for (let c = 0; c < candidates.length; c++) {
                                                            tx.addIceCandidate(candidates[c])
                                                                .then(() => {
                                                                    console.log("Added remote candidate", candidates[c]);
                                                                })
                                                                .catch((e) => {
                                                                    console.error("unable to add candidate", e);
                                                                })
                                                        }
                                                    }
                                                }
                                            }
                                        };
                                        xhr3.open('GET', 'http://localhost:8080/' + tx.id + '/candidate', true);
                                        xhr3.send();
                                    }, 500)
                                })
                                .catch((e) => {
                                    console.error(e);
                                })
                        }
                    };

                    if (tx.id) {
                        xhr.open('POST', 'http://localhost:8080/' + tx.id + '/connection', true);
                    } else {
                        xhr.open('POST', 'http://localhost:8080/connection', true);
                    }

                    xhr.send(JSON.stringify(offer));
                })
                .catch((e) => {
                    console.error(e);
                })
        })
        .catch((e) => {
            console.error(e);
        })
}

function init_player() {
    console.log("Starting Safari Test Player");

    let container = document.getElementById("content");

    navigator.mediaDevices.getUserMedia({
        video: true,
        audio: true
    })
        .then((s) => {
            let local = document.createElement("video");

            local.style.width = "50%";
            local.style.height = "100%";
            local.style.padding = "0";
            local.style.margin = "0";
            local.style.display = "inline-block";
            local.autoplay = true;
            local.playsInline = true;

            container.appendChild(local);

            local.muted = true;
            local.srcObject = s;
            local.play();

            remote = document.createElement("video");

            remote.style.width = "50%";
            remote.style.height = "100%";
            remote.style.padding = "0";
            remote.style.margin = "0";
            remote.style.display = "inline-block";
            remote.autoplay = true;
            remote.playsInline = true;

            container.appendChild(remote);

            let pc = createConnection();

            s.getTracks().forEach((t) => {
                pc.addTrack(t, s);
            });

            startConnection(pc);
        })
        .catch((e) =>{
            console.log("error getting user media", e);
        })
}