package main

import (
	"net/http"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/pion/interceptor"
	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v3"
	log "github.com/sirupsen/logrus"
)

var (
	connections map[string]*webrtc.PeerConnection
	candidates map[string][]*webrtc.ICECandidate
)

func httpCreateConnection(c *gin.Context) {
	log.Info("incoming new connection request")

	var sdp webrtc.SessionDescription

	if err := c.BindJSON(&sdp); err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Error("error parsing JSON")
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	ice := []webrtc.ICEServer{
		{
			URLs: []string{
				"stun:mercury.haia.live",
			},
			Username: "haia",
			Credential: "haia",
		},
	}

	m := &webrtc.MediaEngine{}

	if err := m.RegisterDefaultCodecs(); err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Error("unable to register default codecs")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	i := &interceptor.Registry{}
	if err := webrtc.RegisterDefaultInterceptors(m, i); err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Error("unable to register default interceptors")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	api := webrtc.NewAPI(webrtc.WithMediaEngine(m), webrtc.WithInterceptorRegistry(i))

	pc, err := api.NewPeerConnection(webrtc.Configuration{
		ICEServers: ice,
	})
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Error("unable to create peer connection")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	ID := uuid.New().String()
	connections[ID] = pc

	outputTrack, err := webrtc.NewTrackLocalStaticRTP(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeVP8}, "video", "pion")
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Error("unable to create output track")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	rtpSender, err := pc.AddTrack(outputTrack)
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Error("unable to add track")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	go func() {
		rtcpBuf := make([]byte, 1500)
		for {
			if _, _, err := rtpSender.Read(rtcpBuf); err != nil {
				log.WithFields(log.Fields{
					"error": err,
				}).Error("unable to read from rtp sender")
				return
			}
		}
	}()

	pc.OnTrack(func(remote *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		log.WithFields(log.Fields{
			"receiver": *receiver,
		}).Info("OnTrack")

		go func() {
			ticker := time.NewTicker(time.Second * 3)
			for range ticker.C {
				err := pc.WriteRTCP([]rtcp.Packet{&rtcp.PictureLossIndication{MediaSSRC:uint32(remote.SSRC())}})
				if err != nil {
					log.WithFields(log.Fields{
						"error": err,
					}).Error("unable to write rtcp packet")
				}
			}
		}()

		for {
			rtp, _, err := remote.ReadRTP()
			if err != nil {
				log.WithFields(log.Fields{
					"error": err,
				}).Error("unable to read remote rtp")
				return
			}

			if err := outputTrack.WriteRTP(rtp); err != nil {
				log.WithFields(log.Fields{
					"error": err,
				}).Error("unable to write rtp")
				return
			}
		}
	})

	pc.OnSignalingStateChange(func(state webrtc.SignalingState) {
		log.WithFields(log.Fields{
			"state": state,
		}).Info("OnSignalingStateChange")
	})

	pc.OnICEGatheringStateChange(func(state webrtc.ICEGathererState) {
		log.WithFields(log.Fields{
			"state": state,
		}).Info("OnICEGatheringStateChange")
	})

	pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		log.WithFields(log.Fields{
			"state": state,
		}).Info("OnConnectionStateChange")

		if state == webrtc.PeerConnectionStateFailed {
			delete(connections, ID)
		}
	})

	pc.OnICEConnectionStateChange(func(state webrtc.ICEConnectionState) {
		log.WithFields(log.Fields{
			"state": state,
		}).Info("OnICEConnectionStateChange")
	})

	pc.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate == nil {
			candidates[ID] = append(candidates[ID], candidate)
			log.Info("nil candidate")
			return
		}

		candidates[ID] = append(candidates[ID], candidate)

		log.WithFields(log.Fields{
			"candidate": *candidate,
		}).Info("OnICECandidate")
	})

	pc.OnDataChannel(func(channel *webrtc.DataChannel) {
		log.WithFields(log.Fields{
			"chanell": *channel,
		}).Info("OnDataChannel")
	})

	pc.OnNegotiationNeeded(func() {
		log.Info("OnNegotiationNeeded")
	})

	if err := pc.SetRemoteDescription(sdp); err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Error("unable to set remote description")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Error("unable to create answer")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	if err := pc.SetLocalDescription(answer); err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Error("unable to set local description")
	}

	c.JSON(http.StatusOK, gin.H{
		"id": ID,
		"answer": answer,
	})
}

func httpUpdateConnection(c *gin.Context) {
	log.Warn("updating connection")

	ID := c.Param("ID")
	if ID == "" {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	pc, ok := connections[ID]
	if !ok {
		log.WithFields(log.Fields{
			"ID": ID,
		}).Error("unable to find connection for ID")
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	var sdp webrtc.SessionDescription

	if err := c.BindJSON(&sdp); err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Error("error parsing JSON")
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	if err := pc.SetRemoteDescription(sdp); err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Error("unable to set remote description")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Error("unable to create answer")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	if err := pc.SetLocalDescription(answer); err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Error("unable to set local description")
	}

	c.JSON(http.StatusOK, gin.H{
		"id": ID,
		"answer": answer,
	})
}

func httpUpdateCandidate(c *gin.Context) {
	ID := c.Param("ID")
	if ID == "" {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	pc, ok := connections[ID]
	if !ok {
		log.WithFields(log.Fields{
			"ID": ID,
		}).Error("unable to find connection for ID")
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	var candidate webrtc.ICECandidateInit

	if err := c.BindJSON(&candidate); err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Error("unable to parse JSON")
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	log.WithFields(log.Fields{
		"candidate": candidate,
	}).Info("adding remote candidate")

	if err := pc.AddICECandidate(candidate); err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Error("unable to add ICE candidate")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	c.Status(http.StatusOK)
}

func httpGetCandidate(c *gin.Context) {
	ID := c.Param("ID")
	if ID == "" {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	_, ok := candidates[ID]
	if !ok {
		log.WithFields(log.Fields{
			"ID": ID,
		}).Error("unable to find connection for ID")
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	candidates := candidates[ID]

	if len(candidates) == 0 {
		c.Status(http.StatusNoContent)
		return
	}

	c.JSON(http.StatusOK, candidates)
}

func httpGetConnections(c *gin.Context) {
	var conns []string

	for k := range connections {
		conns = append(conns, k)
	}

	if len(conns) == 0 {
		c.JSON(http.StatusOK, []string{})
		return
	}

	c.JSON(http.StatusOK, conns)
}

func main() {
	connections = make(map[string]*webrtc.PeerConnection)
	candidates = make(map[string][]*webrtc.ICECandidate)

	e := gin.New()

	e.Use(cors.Default())

	e.GET("/connection", httpGetConnections)
	e.POST("/connection", httpCreateConnection)
	e.POST("/:ID/connection", httpUpdateConnection)
	e.POST("/:ID/candidate", httpUpdateCandidate)
	e.GET("/:ID/candidate", httpGetCandidate)

	log.Fatal(e.Run("0.0.0.0:8080"))
}
