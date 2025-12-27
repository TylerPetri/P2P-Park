package parknode

import (
	"encoding/json"
	"time"

	"p2p-park/internal/app/grants"
	"p2p-park/internal/proto"
)

const (
	grantSyncRecentIDs = 64
	grantSyncLimit     = 1000
	grantSyncOverlap   = 5 * time.Minute
)

func (a *App) initiateGrantSync(peerID string) {
	if a.GrantStore == nil {
		return
	}
	maxTS, err := a.GrantStore.MaxTimestamp()
	if err != nil {
		a.logf("grant store max ts: %v", err)
		return
	}
	recent, err := a.GrantStore.RecentGrantIDs(grantSyncRecentIDs)
	if err != nil {
		a.logf("grant store recent ids: %v", err)
		return
	}

	sum := proto.GrantSyncSummary{
		MaxTimestamp:   maxTS,
		RecentGrantIDs: recent,
	}

	body, _ := json.Marshal(sum)
	_ = a.Node.SendToPeer(peerID, proto.Envelope{
		Type:    proto.MsgGrantSyncSummary,
		FromID:  a.Node.ID(),
		Payload: body,
	})
}

func (a *App) handleGrantSyncSummary(env proto.Envelope) {
	if a.GrantStore == nil {
		return
	}
	var sum proto.GrantSyncSummary
	if err := json.Unmarshal(env.Payload, &sum); err != nil {
		return
	}

	localMax, err := a.GrantStore.MaxTimestamp()
	if err != nil {
		return
	}

	// If remote is ahead of us, request missing grants.
	if sum.MaxTimestamp > localMax {
		since := localMax - int64(grantSyncOverlap.Seconds())
		if since < 0 {
			since = 0
		}
		req := proto.GrantSyncRequest{SinceTimestamp: since, Limit: grantSyncLimit}
		body, _ := json.Marshal(req)
		_ = a.Node.SendToPeer(env.FromID, proto.Envelope{
			Type:    proto.MsgGrantSyncRequest,
			FromID:  a.Node.ID(),
			Payload: body,
		})
	}

}

func (a *App) handleGrantSyncRequest(env proto.Envelope) {
	if a.GrantStore == nil {
		return
	}
	var req proto.GrantSyncRequest
	if err := json.Unmarshal(env.Payload, &req); err != nil {
		return
	}
	grs, err := a.GrantStore.ListSince(req.SinceTimestamp, req.Limit)
	if err != nil {
		return
	}
	resp := proto.GrantSyncResponse{Grants: grs}
	body, _ := json.Marshal(resp)
	_ = a.Node.SendToPeer(env.FromID, proto.Envelope{
		Type:    proto.MsgGrantSyncResponse,
		FromID:  a.Node.ID(),
		Payload: body,
	})
}

func (a *App) handleGrantSyncResponse(env proto.Envelope) {
	if a.GrantStore == nil {
		return
	}
	var resp proto.GrantSyncResponse
	if err := json.Unmarshal(env.Payload, &resp); err != nil {
		return
	}

	for _, g := range resp.Grants {
		// verify first to avoid persisting garbage
		if err := grants.VerifyGrant(g); err != nil {
			continue
		}
		if a.Ledger.ApplyGrant(g) {
			_, _ = a.GrantStore.Put(g)
		}
	}
}
