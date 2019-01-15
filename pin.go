package httpapi

import (
	"context"
	"encoding/json"
	"github.com/ipfs/go-cid"
	"github.com/pkg/errors"

	"github.com/ipfs/go-ipfs/core/coreapi/interface"
	caopts "github.com/ipfs/go-ipfs/core/coreapi/interface/options"
)

type PinAPI HttpApi

type pinRefKeyObject struct {
	Type string
}

type pinRefKeyList struct {
	Keys map[string]pinRefKeyObject
}

type pin struct {
	path iface.ResolvedPath
	typ  string
}

func (p *pin) Path() iface.ResolvedPath {
	return p.path
}

func (p *pin) Type() string {
	return p.typ
}

func (api *PinAPI) Add(ctx context.Context, p iface.Path, opts ...caopts.PinAddOption) error {
	options, err := caopts.PinAddOptions(opts...)
	if err != nil {
		return err
	}

	return api.core().request("pin/add", p.String()).
		Option("recursive", options.Recursive).Exec(ctx, nil)
}

func (api *PinAPI) Ls(ctx context.Context, opts ...caopts.PinLsOption) ([]iface.Pin, error) {
	options, err := caopts.PinLsOptions(opts...)
	if err != nil {
		return nil, err
	}

	var out pinRefKeyList
	err = api.core().request("pin/ls").
		Option("type", options.Type).Exec(ctx, &out)
	if err != nil {
		return nil, err
	}

	pins := make([]iface.Pin, 0, len(out.Keys))
	for hash, p := range out.Keys {
		c, err := cid.Parse(hash)
		if err != nil {
			return nil, err
		}
		pins = append(pins, &pin{typ: p.Type, path: iface.IpldPath(c)})
	}

	return pins, nil
}

func (api *PinAPI) Rm(ctx context.Context, p iface.Path) error {
	return api.core().request("pin/rm", p.String()).Exec(ctx, nil)
}

func (api *PinAPI) Update(ctx context.Context, from iface.Path, to iface.Path, opts ...caopts.PinUpdateOption) error {
	options, err := caopts.PinUpdateOptions(opts...)
	if err != nil {
		return err
	}

	return api.core().request("pin/update").
		Option("unpin", options.Unpin).Exec(ctx, nil)
}

type pinVerifyRes struct {
	Cid      string
	JOk       bool `json:"Ok"`
	JBadNodes []*badNode `json:"BadNodes,omitempty"`
}

func (r *pinVerifyRes) Ok() bool {
	return r.JOk
}

func (r *pinVerifyRes) BadNodes() []iface.BadPinNode {
	out := make([]iface.BadPinNode, len(r.JBadNodes))
	for i, n := range r.JBadNodes {
		out[i] = n
	}
	return out
}

type badNode struct {
	Cid string
	JErr string `json:"Err"`
}

func (n *badNode) Path() iface.ResolvedPath {
	c, err := cid.Parse(n.Cid)
	if err != nil {
		return nil // todo: handle this better
	}
	return iface.IpldPath(c)
}

func (n *badNode) Err() error {
	if n.JErr != "" {
		return errors.New(n.JErr)
	}
	if _, err := cid.Parse(n.Cid); err != nil {
		return err
	}
	return nil
}

func (api *PinAPI) Verify(ctx context.Context) (<-chan iface.PinStatus, error) {
	resp, err := api.core().request("pin/verify").Option("verbose", true).Send(ctx)
	if err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, resp.Error
	}
	res := make(chan iface.PinStatus)

	go func() {
		defer resp.Close()
		defer close(res)
		dec := json.NewDecoder(resp.Output)
		for {
			var out pinVerifyRes
			if err := dec.Decode(&out); err != nil {
				return // todo: handle non io.EOF somehow
			}

			select {
			case res <- &out:
			case <-ctx.Done():
				return
			}
		}
	}()

	return res, nil
}

func (api *PinAPI) core() *HttpApi {
	return (*HttpApi)(api)
}
