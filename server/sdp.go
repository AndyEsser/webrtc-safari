package server

const (
	SessionTypeOffer = "offer"
	SessionTypeAnswer = "answer"
)

type SessionDescriptor struct {
	SDP string `json:"sdp"`
	Type string `json:"type"`
}