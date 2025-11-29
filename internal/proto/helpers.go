package proto

import "encoding/json"

func MustMarshal(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

// EncodeSnapshotCanonical encodes a PointsSnapshot in a canonical way
// so signing/verifying uses the exact same bytes.
func EncodeSnapshotCanonical(s PointsSnapshot) ([]byte, error) {
	return json.Marshal(s)
}
