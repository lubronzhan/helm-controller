package reconcile

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sync"

	helmaction "helm.sh/helm/v4/pkg/action"

	"github.com/fluxcd/helm-controller/internal/action"
)

// Best-effort reconstruction of fluxcd/helm-controller#1583's proposed
// "templateDigest" drift-detection mode: full re-render via a real Helm
// dry-run (server-side, so .Capabilities.APIVersions reflects live cluster
// state), hashed and compared against the last-observed value. This is NOT
// the real upstream fix -- no persisted CRD status field, no
// generation-gating, in-memory only, for spike verification purposes.
var (
	templateDigestMu sync.Mutex
	templateDigests  = map[string]string{}
)

func checkTemplateDigestDrift(ctx context.Context, cfg *action.ConfigFactory, req *Request) (bool, error) {
	rel, err := action.Upgrade(ctx, cfg.Build(nil), req.Object, req.Chart, req.Values, func(u *helmaction.Upgrade) {
		u.DryRunStrategy = helmaction.DryRunServer
	})
	if err != nil {
		return false, err
	}

	sum := sha256.Sum256([]byte(rel.Manifest))
	digest := hex.EncodeToString(sum[:])

	key := req.Object.Namespace + "/" + req.Object.Name
	templateDigestMu.Lock()
	defer templateDigestMu.Unlock()
	prev, ok := templateDigests[key]
	templateDigests[key] = digest
	if !ok {
		return false, nil
	}
	return prev != digest, nil
}
