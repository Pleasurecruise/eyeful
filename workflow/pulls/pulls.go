package pulls

import (
	"crypto/sha256"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/MoeForge/moejs"
)

//go:embed core.js
var source string

func Load() (*Core, error) {
	mod, err := moejs.Compile("core.js", source)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrCore, err)
	}
	return &Core{mod: mod}, nil
}

func call[T []FileChange | string | json.RawMessage | checked](c *Core, name string, args ...json.RawMessage) (T, error) {
	var out T
	hook, err := c.mod.Hook(name)
	if err != nil {
		return out, fmt.Errorf("%w: %s: %w", ErrCore, name, err)
	}
	rt := moejs.NewRuntime(moejs.Options{})
	crypto, err := rt.ParseJSON([]byte(`{"subtle":{}}`))
	if err != nil {
		return out, fmt.Errorf("%w: %w", ErrCore, err)
	}
	subtle, err := rt.Get(crypto, "subtle")
	if err != nil {
		return out, fmt.Errorf("%w: %w", ErrCore, err)
	}
	digest := rt.Function("digest", 2, func(_ *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
		if len(args) != 2 {
			return moejs.Value{}, fmt.Errorf("%w: digest takes an algorithm and the data, got %d arguments", ErrCore, len(args))
		}
		var data []byte
		if err := rt.ToGoInto(args[1], &data); err != nil {
			return moejs.Value{}, fmt.Errorf("%w: digest: %w", ErrCore, err)
		}
		sum := sha256.Sum256(data)
		return rt.FromGo(sum[:])
	})
	if err := errors.Join(subtle.AsObject().SetProp(rt.Realm(), rt.Realm().KeyFromGoString("digest"), digest), rt.SetGlobal("crypto", crypto)); err != nil {
		return out, fmt.Errorf("%w: %w", ErrCore, err)
	}
	if err := rt.Load(c.mod); err != nil {
		return out, fmt.Errorf("%w: %w", ErrCore, err)
	}
	values := make([]moejs.Value, len(args))
	for i, a := range args {
		if values[i], err = rt.ParseJSON(a); err != nil {
			return out, fmt.Errorf("%w: %s argument %d: %w", ErrCore, name, i, err)
		}
	}
	res, err := rt.Call(hook, values...)
	if err != nil {
		return out, fmt.Errorf("%w: %s: %w", ErrCore, name, err)
	}
	if state, v, ok := moejs.PromiseResult(res); ok {
		if state != moejs.PromiseFulfilled {
			var reason string
			message, err := rt.Get(v, "message")
			if err == nil {
				err = rt.Unmarshal(message, &reason)
			}
			return out, errors.Join(fmt.Errorf("%w: %s did not finish: %s", ErrCore, name, reason), err)
		}
		res = v
	}
	if err := rt.Unmarshal(res, &out); err != nil {
		return out, fmt.Errorf("%w: %s result: %w", ErrCore, name, err)
	}
	return out, nil
}

func (c *Core) ParsePatch(patch string) ([]FileChange, error) {
	arg, err := json.Marshal(patch)
	if err != nil {
		return nil, fmt.Errorf("%w: patch: %w", ErrCore, err)
	}
	return call[[]FileChange](c, "parsePatch", arg)
}

func (c *Core) Prompt(d DiffsPayload, patchPath string) (string, error) {
	diff, err := json.Marshal(d)
	if err != nil {
		return "", fmt.Errorf("%w: diff: %w", ErrCore, err)
	}
	path, err := json.Marshal(patchPath)
	if err != nil {
		return "", fmt.Errorf("%w: patch path: %w", ErrCore, err)
	}
	return call[string](c, "prompt", diff, path)
}

func (c *Core) AnalysisJSONSchema() (json.RawMessage, error) {
	return call[json.RawMessage](c, "analysisJsonSchema")
}

func (c *Core) Check(d DiffsPayload, a Analysis) (Analysis, string, error) {
	diff, err := json.Marshal(d)
	if err != nil {
		return Analysis{}, "", fmt.Errorf("%w: diff: %w", ErrCore, err)
	}
	answer, err := json.Marshal(a)
	if err != nil {
		return Analysis{}, "", fmt.Errorf("%w: answer: %w", ErrCore, err)
	}
	r, err := call[checked](c, "check", diff, answer)
	return r.Analysis, r.Problem, err
}
