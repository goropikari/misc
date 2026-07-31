package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"
)

const (
	defaultPageSize     = 1000
	openFGAReadPageSize = 100
)

type config struct {
	APIURL, StoreID, Token, Addr string
	PageSize                     int
}

type tuple struct {
	User     string `json:"user"`
	Relation string `json:"relation"`
	Object   string `json:"object"`
}

type readResponse struct {
	Tuples []struct {
		Key tuple `json:"key"`
	} `json:"tuples"`
	ContinuationToken string `json:"continuation_token"`
}

type checkResponse struct {
	Allowed bool          `json:"allowed"`
	Paths   []string      `json:"paths,omitempty"`
	Regions *checkRegions `json:"regions,omitempty"`
}

type checkRegions struct {
	SourceNodes []string `json:"source_nodes"`
	TargetNodes []string `json:"target_nodes"`
}

type expandResponse struct {
	Tree json.RawMessage `json:"tree"`
}

type modelResponse struct {
	AuthorizationModel authorizationModel `json:"authorization_model"`
}

type modelListResponse struct {
	AuthorizationModels []authorizationModel `json:"authorization_models"`
}

type authorizationModel struct {
	ID              string           `json:"id"`
	SchemaVersion   string           `json:"schema_version"`
	TypeDefinitions []typeDefinition `json:"type_definitions"`
}

type typeDefinition struct {
	Type      string                     `json:"type"`
	Relations map[string]json.RawMessage `json:"relations"`
	Metadata  json.RawMessage            `json:"metadata"`
}

type relation struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	Expression string `json:"expression"`
}

type modelData struct {
	ID        string           `json:"id"`
	Schema    string           `json:"schema"`
	Types     []typeDefinition `json:"types"`
	Relations []relation       `json:"relations"`
	DSL       string           `json:"dsl"`
}

type pageData struct {
	Config                 config
	Tuples                 []tuple
	Model                  modelData
	ModelJSON              template.JS
	TuplesJSON             template.JS
	ContinuationToken      string
	NextURL, RefreshURL    string
	Object, Relation, User string
	Error                  string
	UpdatedAt              string
	PageSize               int
	CSS                    template.CSS
	JS                     template.JS
}

type app struct {
	cfg  config
	tpl  *template.Template
	http *http.Client
}

func main() {
	addr := flag.String("addr", envOr("OPENFGA_VISUALIZER_ADDR", "127.0.0.1:8080"), "HTTP listen address")

	flag.Parse()

	cfg := config{
		APIURL:  strings.TrimRight(os.Getenv("OPENFGA_API_URL"), "/"),
		StoreID: os.Getenv("OPENFGA_STORE_ID"),
		Token:   os.Getenv("OPENFGA_AUTH_TOKEN"),
		Addr:    *addr, PageSize: defaultPageSize,
	}
	if err := cfg.validate(); err != nil {
		log.Fatal(err)
	}

	tpl := template.Must(template.New("page").Funcs(template.FuncMap{"js": func(v template.JS) template.JS { return v }}).Parse(strings.ReplaceAll(pageTemplateText, legendHTML, "")))
	server := &app{cfg: cfg, tpl: tpl, http: &http.Client{Timeout: 20 * time.Second}}
	mux := http.NewServeMux()
	mux.HandleFunc("/", server.handle)
	mux.HandleFunc("/check", server.handleCheck)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	})
	log.Printf("FGA Lens listening on http://%s", cfg.Addr)
	httpServer := &http.Server{Addr: cfg.Addr, Handler: logRequests(mux), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 20 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}
	log.Fatal(httpServer.ListenAndServe())
}

//nolint:cyclop // The endpoint validates, checks, expands, and serializes one request.
func (a *app) handleCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	key := tuple{User: r.FormValue("user"), Relation: r.FormValue("relation"), Object: r.FormValue("object")}
	if key.User == "" || key.Relation == "" || key.Object == "" {
		http.Error(w, "user, relation, and object are required", http.StatusBadRequest)
		return
	}

	model, err := a.latestModel(r.Context())
	if err == nil {
		var result checkResponse

		payload := map[string]any{
			"authorization_model_id": model.ID,
			"tuple_key":              key,
		}

		body, marshalErr := json.Marshal(payload)
		if marshalErr != nil {
			err = marshalErr
		} else {
			err = a.request(r.Context(), http.MethodPost, "/stores/"+url.PathEscape(a.cfg.StoreID)+"/check", bytes.NewReader(body), &result)
			if err == nil {
				if result.Allowed {
					result.Paths, err = a.expandPaths(r.Context(), model.ID, key)
				} else {
					result.Regions, err = a.checkRegions(r.Context(), key)
				}

				if err != nil {
					http.Error(w, err.Error(), http.StatusBadGateway)
					return
				}

				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(result)

				return
			}
		}
	}

	http.Error(w, err.Error(), http.StatusBadGateway)
}

func (a *app) checkRegions(ctx context.Context, key tuple) (*checkRegions, error) {
	all, err := a.readAllTuples(ctx)
	if err != nil {
		return nil, err
	}

	outgoing := make(map[string][]string)
	incoming := make(map[string][]string)

	addEdge := func(from, to string) {
		outgoing[from] = append(outgoing[from], to)
		incoming[to] = append(incoming[to], from)
	}
	for _, item := range all {
		addEdge(item.User, item.Object)

		if hash := strings.IndexByte(item.User, '#'); hash > 0 {
			addEdge(item.User[:hash], item.User)
		}
	}

	return &checkRegions{
		SourceNodes: traverseNodes(key.User, outgoing),
		TargetNodes: traverseNodes(key.Object, incoming),
	}, nil
}

func (a *app) readAllTuples(ctx context.Context) ([]tuple, error) {
	all := make([]tuple, 0)

	continuation := ""
	for {
		items, next, err := a.readTuplePage(ctx, nil, continuation)
		if err != nil {
			return nil, err
		}

		all = append(all, items...)
		if next == "" {
			return all, nil
		}

		continuation = next
	}
}

func traverseNodes(start string, edges map[string][]string) []string {
	seen := map[string]bool{start: true}

	queue := []string{start}
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]

		for _, next := range edges[node] {
			if !seen[next] {
				seen[next] = true
				queue = append(queue, next)
			}
		}
	}

	result := make([]string, 0, len(seen))
	for node := range seen {
		result = append(result, node)
	}

	sort.Strings(result)

	return result
}

func (a *app) expandPaths(ctx context.Context, modelID string, key tuple) ([]string, error) {
	payload, err := json.Marshal(map[string]any{
		"authorization_model_id": modelID,
		"tuple_key":              map[string]string{"relation": key.Relation, "object": key.Object},
	})
	if err != nil {
		return nil, err
	}

	var response expandResponse
	if err := a.request(ctx, http.MethodPost, "/stores/"+url.PathEscape(a.cfg.StoreID)+"/expand", bytes.NewReader(payload), &response); err != nil {
		return nil, err
	}

	value := any(nil)
	if len(response.Tree) > 0 {
		if err := json.Unmarshal(response.Tree, &value); err != nil {
			return nil, err
		}
	}

	return a.expandProofPaths(ctx, modelID, key, value)
}

type expandBranch struct {
	Path     []string
	Endpoint string
}

func (a *app) expandProofPaths(ctx context.Context, modelID string, key tuple, tree any) ([]string, error) {
	paths, err := a.expandProofUserset(ctx, modelID, key.Object+"#"+key.Relation, key.User, nil, tree, map[string]bool{})
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool)

	result := make([]string, 0, len(paths))
	for _, path := range paths {
		value := strings.Join(path, " → ")
		if !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}

	return result, nil
}

func (a *app) expandProofUserset(ctx context.Context, modelID, userset, user string, prefix []string, tree any, visited map[string]bool) ([][]string, error) {
	if visited[userset] {
		return nil, nil
	}

	visited[userset] = true
	paths := make([][]string, 0)

	for _, branch := range collectExpandBranches(tree, prefix) {
		if branch.Endpoint == user {
			paths = append(paths, append(branch.Path, user))
			continue
		}

		object, relation, ok := splitUserset(branch.Endpoint)
		if !ok {
			continue
		}

		childTree, err := a.readExpandTree(ctx, modelID, object, relation)
		if err != nil {
			return nil, err
		}

		childPaths, err := a.expandProofUserset(ctx, modelID, branch.Endpoint, user, branch.Path, childTree, visited)
		if err != nil {
			return nil, err
		}

		paths = append(paths, childPaths...)
	}

	return paths, nil
}

func (a *app) readExpandTree(ctx context.Context, modelID, object, relation string) (any, error) {
	payload, err := json.Marshal(map[string]any{
		"authorization_model_id": modelID,
		"tuple_key":              map[string]string{"relation": relation, "object": object},
	})
	if err != nil {
		return nil, err
	}

	var response expandResponse
	if err := a.request(ctx, http.MethodPost, "/stores/"+url.PathEscape(a.cfg.StoreID)+"/expand", bytes.NewReader(payload), &response); err != nil {
		return nil, err
	}

	var tree any
	if err := json.Unmarshal(response.Tree, &tree); err != nil {
		return nil, err
	}

	return tree, nil
}

func splitUserset(value string) (string, string, bool) {
	separator := strings.LastIndexByte(value, '#')
	if separator <= 0 || separator == len(value)-1 {
		return "", "", false
	}

	return value[:separator], value[separator+1:], true
}

//nolint:cyclop,gocognit // Expand trees have several leaf and branch shapes.
func collectExpandBranches(value any, prefix []string) []expandBranch {
	object, ok := value.(map[string]any)
	if !ok {
		return nil
	}

	path := append([]string{}, prefix...)
	if name, ok := object["name"].(string); ok && name != "" && (len(path) == 0 || path[len(path)-1] != name) {
		path = append(path, name)
	}

	if leaf, ok := object["leaf"].(map[string]any); ok {
		if users, ok := leaf["users"].(map[string]any); ok {
			entries, _ := users["users"].([]any)

			branches := make([]expandBranch, 0, len(entries))
			for _, entry := range entries {
				if endpoint, ok := entry.(string); ok {
					branches = append(branches, expandBranch{Path: path, Endpoint: endpoint})
				}
			}

			return branches
		}

		if userset, ok := leaf["tupleToUserset"].(map[string]any); ok {
			computed, _ := userset["computed"].([]any)

			branches := make([]expandBranch, 0, len(computed))
			for _, item := range computed {
				if computedUser, ok := item.(map[string]any); ok {
					if endpoint, ok := computedUser["userset"].(string); ok {
						branches = append(branches, expandBranch{Path: path, Endpoint: endpoint})
					}
				}
			}

			return branches
		}
	}

	branches := make([]expandBranch, 0)

	if nodes, ok := object["nodes"].([]any); ok {
		for _, node := range nodes {
			branches = append(branches, collectExpandBranches(node, path)...)
		}
	}

	if root, ok := object["root"].(map[string]any); ok {
		branches = append(branches, collectExpandBranches(root, path)...)
	}

	for _, name := range []string{"union", "intersection", "difference"} {
		if group, ok := object[name].(map[string]any); ok {
			branches = append(branches, collectExpandBranches(group, path)...)
		}
	}

	return branches
}

func (c config) validate() error {
	for name, value := range map[string]string{"OPENFGA_API_URL": c.APIURL, "OPENFGA_STORE_ID": c.StoreID, "OPENFGA_AUTH_TOKEN": c.Token} {
		if value == "" {
			return fmt.Errorf("%s is required", name)
		}
	}

	if _, err := url.ParseRequestURI(c.APIURL); err != nil {
		return fmt.Errorf("OPENFGA_API_URL is invalid: %w", err)
	}

	return nil
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}

	return fallback
}

func (a *app) handle(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	data := pageData{Config: a.cfg, PageSize: defaultPageSize, Object: r.FormValue("object"), Relation: r.FormValue("relation"), User: r.FormValue("user"), UpdatedAt: time.Now().Format("2006-01-02 15:04:05"), CSS: safeCSS(), JS: safeJS()}
	t := tuple{Object: data.Object, Relation: data.Relation, User: data.User}

	model, err := a.latestModel(r.Context())
	if err == nil {
		data.Model = buildModelData(model)
		data.ModelJSON = jsonTemplate(data.Model)
		data.Tuples, data.ContinuationToken, err = a.readTuples(r.Context(), t, "", model.TypeDefinitions)
		data.TuplesJSON = jsonTemplate(data.Tuples)
	}

	if err != nil {
		data.Error = err.Error()
	}

	data.RefreshURL = "/?" + values(data)
	if data.ContinuationToken != "" {
		data.NextURL = "/?" + values(data) + "&continuation_token=" + url.QueryEscape(data.ContinuationToken)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if err := a.tpl.ExecuteTemplate(w, "page", data); err != nil {
		log.Printf("render: %v", err)
	}
}

func values(d pageData) string {
	v := url.Values{}
	v.Set("object", d.Object)
	v.Set("relation", d.Relation)
	v.Set("user", d.User)

	return v.Encode()
}

func (a *app) latestModel(ctx context.Context) (authorizationModel, error) {
	var out modelListResponse

	err := a.request(ctx, http.MethodGet, "/stores/"+url.PathEscape(a.cfg.StoreID)+"/authorization-models?page_size=100", nil, &out)
	if err != nil {
		return authorizationModel{}, err
	}

	if len(out.AuthorizationModels) == 0 {
		return authorizationModel{}, fmt.Errorf("OpenFGA store has no authorization model")
	}

	latest := out.AuthorizationModels[0]
	for _, candidate := range out.AuthorizationModels[1:] {
		if candidate.ID > latest.ID {
			latest = candidate
		}
	}

	var detail modelResponse

	err = a.request(ctx, http.MethodGet, "/stores/"+url.PathEscape(a.cfg.StoreID)+"/authorization-models/"+url.PathEscape(latest.ID), nil, &detail)
	if err != nil {
		return authorizationModel{}, err
	}

	return detail.AuthorizationModel, nil
}

//nolint:cyclop // Filtering across object types and pagination requires these branches.
func (a *app) readTuples(ctx context.Context, filter tuple, continuation string, types []typeDefinition) ([]tuple, string, error) {
	if filter.User != "" && filter.Object == "" && filter.Relation == "" {
		items, err := a.readTuplesForUserGraph(ctx, filter.User)
		return items, "", err
	}
	if filter.Object != "" && filter.User == "" && filter.Relation == "" {
		items, err := a.readTuplesForObjectGraph(ctx, filter.Object)
		return items, "", err
	}

	objects := []string{filter.Object}
	if filter.Object == "" && (filter.User != "" || filter.Relation != "") {
		objects = make([]string, 0, len(types))
		for _, definition := range types {
			objects = append(objects, definition.Type+":")
		}
	}

	all := make([]tuple, 0, a.cfg.PageSize)

	for _, object := range objects {
		filter.Object = object

		items, next, err := a.readTuplesForObject(ctx, filter, continuation)
		if err != nil {
			return nil, "", err
		}

		all = append(all, items...)
		if len(all) >= a.cfg.PageSize {
			return all[:min(len(all), a.cfg.PageSize)], "", nil
		}

		if next != "" {
			return all, next, nil
		}
	}

	return all, "", nil
}

// readTuplesForUserGraph loads tuples reachable from a user by following
// user-to-object edges, including userset references such as group:engineering#member.
//
//nolint:cyclop // The directed traversal handles pagination, usersets, and cycle guards.
func (a *app) readTuplesForUserGraph(ctx context.Context, user string) ([]tuple, error) {
	all, err := a.readAllTuples(ctx)
	if err != nil {
		return nil, err
	}

	byUser := make(map[string][]int)
	for i, item := range all {
		byUser[item.User] = append(byUser[item.User], i)
	}

	seenNodes := map[string]bool{}
	seenTuples := map[int]bool{}
	queue := []string{user}
	seenNodes[user] = true

	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]

		for _, i := range byUser[node] {
			item := all[i]
			seenTuples[i] = true
			enqueue := func(next string) {
				if next != "" && !seenNodes[next] {
					seenNodes[next] = true
					queue = append(queue, next)
				}
			}

			enqueue(item.User)
			enqueue(item.Object)

			if hash := strings.IndexByte(item.User, '#'); hash > 0 {
				enqueue(item.User[:hash])
			}

			enqueue(item.Object + "#" + item.Relation)
		}
	}

	connected := make([]tuple, 0, len(seenTuples))
	for i, item := range all {
		if seenTuples[i] {
			connected = append(connected, item)
		}
	}

	return connected, nil
}

// readTuplesForObjectGraph loads tuples reachable from an object by following
// object-to-user edges in reverse. This makes parent objects and other users
// that point at the searched object visible recursively.
func (a *app) readTuplesForObjectGraph(ctx context.Context, object string) ([]tuple, error) {
	all, err := a.readAllTuples(ctx)
	if err != nil {
		return nil, err
	}

	byObject := make(map[string][]int)
	for i, item := range all {
		byObject[item.Object] = append(byObject[item.Object], i)
	}

	seenNodes := map[string]bool{object: true}
	seenTuples := make(map[int]bool)
	queue := []string{object}
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]

		for _, i := range byObject[node] {
			item := all[i]
			seenTuples[i] = true
			enqueue := func(next string) {
				if next != "" && !seenNodes[next] {
					seenNodes[next] = true
					queue = append(queue, next)
				}
			}

			enqueue(item.User)
			if hash := strings.IndexByte(item.User, '#'); hash > 0 {
				// A userset is backed by tuples for its base object. Follow
				// that base as well so the graph remains connected upstream.
				enqueue(item.User[:hash])
			}
		}
	}

	connected := make([]tuple, 0, len(seenTuples))
	for i, item := range all {
		if seenTuples[i] {
			connected = append(connected, item)
		}
	}

	return connected, nil
}

func (a *app) readTuplesForObject(ctx context.Context, filter tuple, continuation string) ([]tuple, string, error) {
	key := map[string]string{}
	if filter.Object != "" {
		key["object"] = filter.Object
	}

	if filter.Relation != "" {
		key["relation"] = filter.Relation
	}

	if filter.User != "" {
		key["user"] = filter.User
	}

	items := make([]tuple, 0, a.cfg.PageSize)
	for len(items) < a.cfg.PageSize {
		page, next, err := a.readTuplePage(ctx, key, continuation)
		if err != nil {
			return nil, "", err
		}

		items = append(items, page...)
		if next == "" || len(page) == 0 {
			return items, "", nil
		}

		continuation = next
	}

	return items, continuation, nil
}

func (a *app) readTuplePage(ctx context.Context, key map[string]string, continuation string) ([]tuple, string, error) {
	body := map[string]any{"page_size": openFGAReadPageSize}
	if len(key) > 0 {
		body["tuple_key"] = key
	}

	if continuation != "" {
		body["continuation_token"] = continuation
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, "", err
	}

	var out readResponse
	if err = a.request(ctx, http.MethodPost, "/stores/"+url.PathEscape(a.cfg.StoreID)+"/read", bytes.NewReader(payload), &out); err != nil {
		return nil, "", err
	}

	items := make([]tuple, 0, len(out.Tuples))
	for _, item := range out.Tuples {
		items = append(items, item.Key)
	}

	return items, out.ContinuationToken, nil
}

func (a *app) request(ctx context.Context, method, path string, body io.Reader, out any) error {
	req, err := http.NewRequestWithContext(ctx, method, a.cfg.APIURL+path, body)
	if err != nil {
		return err
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.cfg.Token)

	resp, err := a.http.Do(req)
	if err != nil {
		return fmt.Errorf("OpenFGA request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("OpenFGA returned %s: %s", resp.Status, strings.TrimSpace(string(b)))
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode OpenFGA response: %w", err)
	}

	return nil
}

func buildModelData(m authorizationModel) modelData {
	d := modelData{ID: m.ID, Schema: m.SchemaVersion, Types: m.TypeDefinitions, DSL: renderDSL(m)}
	for _, td := range m.TypeDefinitions {
		for name, raw := range td.Relations {
			d.Relations = append(d.Relations, relation{Name: name, Type: td.Type, Expression: expression(raw)})
		}
	}

	return d
}

func renderDSL(m authorizationModel) string {
	var b strings.Builder
	b.WriteString("model\n  schema " + m.SchemaVersion + "\n\n")

	for _, definition := range m.TypeDefinitions {
		b.WriteString("type " + definition.Type + "\n")

		if len(definition.Relations) == 0 {
			b.WriteString("\n")
			continue
		}

		b.WriteString("  relations\n")

		names := make([]string, 0, len(definition.Relations))
		for name := range definition.Relations {
			names = append(names, name)
		}

		sort.Strings(names)

		for _, name := range names {
			b.WriteString("    define " + name + ": " + renderRewrite(definition.Relations[name], relationTypes(definition.Metadata, name)) + "\n")
		}

		b.WriteString("\n")
	}

	return strings.TrimSpace(b.String())
}

func relationTypes(metadata json.RawMessage, name string) string {
	var root map[string]any
	if json.Unmarshal(metadata, &root) != nil {
		return ""
	}

	relations, _ := root["relations"].(map[string]any)
	entry, _ := relations[name].(map[string]any)
	items, _ := entry["directly_related_user_types"].([]any)

	values := make([]string, 0, len(items))
	for _, item := range items {
		value, _ := item.(map[string]any)
		typ, _ := value["type"].(string)

		rel, _ := value["relation"].(string)
		if rel != "" {
			typ += "#" + rel
		}

		if typ != "" {
			values = append(values, typ)
		}
	}

	if len(values) == 0 {
		return ""
	}

	return "[" + strings.Join(values, ", ") + "]"
}

//nolint:cyclop // The OpenFGA rewrite grammar has several mutually exclusive operators.
func renderRewrite(raw json.RawMessage, allowed string) string {
	var value map[string]any
	if json.Unmarshal(raw, &value) != nil {
		return ""
	}

	if _, ok := value["this"]; ok {
		if allowed != "" {
			return allowed
		}

		return "..."
	}

	if computed, ok := value["computedUserset"].(map[string]any); ok {
		return stringValue(computed["relation"])
	}

	if userset, ok := value["tupleToUserset"].(map[string]any); ok {
		tupleset, _ := userset["tupleset"].(map[string]any)
		computed, _ := userset["computedUserset"].(map[string]any)

		return stringValue(computed["relation"]) + " from " + stringValue(tupleset["relation"])
	}

	for _, key := range []string{"union", "intersection", "difference"} {
		if group, ok := value[key].(map[string]any); ok {
			children, _ := group["child"].([]any)

			parts := make([]string, 0, len(children))
			for _, child := range children {
				rawChild, _ := json.Marshal(child)
				parts = append(parts, renderRewrite(rawChild, allowed))
			}

			separator := " or "
			if key == "intersection" {
				separator = " and "
			}

			if key == "difference" {
				separator = " but not "
			}

			return strings.Join(parts, separator)
		}
	}

	return "..."
}

func expression(raw json.RawMessage) string {
	var value any
	if json.Unmarshal(raw, &value) != nil {
		return ""
	}

	return expressionValue(value)
}

//nolint:cyclop // The OpenFGA rewrite grammar has several mutually exclusive operators.
func expressionValue(value any) string {
	object, ok := value.(map[string]any)
	if !ok {
		return ""
	}

	if _, ok := object["this"]; ok {
		return "direct"
	}

	if computed, ok := object["computedUserset"].(map[string]any); ok {
		return "computed: " + stringValue(computed["relation"])
	}

	if userset, ok := object["tupleToUserset"].(map[string]any); ok {
		tupleset, _ := userset["tupleset"].(map[string]any)
		computed, _ := userset["computedUserset"].(map[string]any)

		return "from: " + stringValue(tupleset["relation"]) + " → " + stringValue(computed["relation"])
	}

	for _, key := range []string{"union", "intersection", "difference"} {
		if group, ok := object[key].(map[string]any); ok {
			if children, ok := group["child"].([]any); ok {
				for _, child := range children {
					if result := expressionValue(child); result != "" && result != "direct" {
						return result
					}
				}
			}

			return key
		}
	}

	return "model relation"
}

func stringValue(value any) string {
	if text, ok := value.(string); ok {
		return text
	}

	return ""
}

func safeCSS() template.CSS { return template.CSS(pageCSS) } //nolint:gosec // pageCSS is a built-in constant.

func safeJS() template.JS {
	script := pageJS + dynamicJS + checkHighlightJS + checkRegionJS + pagePersistJS + layoutControlJS + checkJS + checkPanelJS + checkPersistJS + tableSortJS + languageJS
	// User/object positions are contextual in OpenFGA, so node colors must not imply a type.
	script = strings.ReplaceAll(script, "const color=n.kind==='user'?'#2f7df6':n.kind==='object'?'#f08a55':'#8d67d8';", "const color='#6f7d87';")
	script = strings.ReplaceAll(script, ";(()=>{const key='fgalens.viewMode'", `;(()=>{const esc=s=>String(s).replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));const key='fgalens.viewMode'`)
	script = strings.ReplaceAll(script, `const data=makeGraph();host.innerHTML=`, `const data=makeGraph();if(mode==='tuple')data.nodes.forEach(n=>{n.x=n.kind==='principal'?140:760});host.innerHTML=`)
	script = strings.ReplaceAll(script, `n.vx+=(W/2-n.x)*.002;n.vy+=(H/2-n.y)*.002;`, `n.vx+=((mode==='tuple'?(n.kind==='principal'?140:760):W/2)-n.x)*.02;n.vy+=(H/2-n.y)*.002;`)
	script = strings.ReplaceAll(script, `<g id="viewport"></g>`, `<defs><marker id="fgalens-arrow" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto"><path d="M0 0L8 4L0 8z" fill="#6f7d87"/></marker></defs><g id="viewport"></g>`)
	script = strings.ReplaceAll(script, `line.setAttribute('stroke-width','1.8');`, `line.setAttribute('stroke-width','1.8');line.setAttribute('marker-end','url(#fgalens-arrow)');`)
	script = strings.ReplaceAll(script, `line.setAttribute('stroke','#8b98a1');`, `line.dataset.from=e.a;line.dataset.to=e.b;line.setAttribute('stroke','#8b98a1');`)
	script = strings.ReplaceAll(script, `g.classList.add('interactive-node');`, `g.classList.add('interactive-node');g.dataset.node=n.id;`)
	script = strings.ReplaceAll(script, `((same(line.dataset.from,part)&&same(line.dataset.to,path[i+1]))||(same(line.dataset.to,part)&&same(line.dataset.from,path[i+1])))`, `((line.dataset.to.indexOf('#')>=0&&same(line.dataset.to,part))||(line.dataset.from.indexOf('#')>=0&&same(line.dataset.from,part))||(same(line.dataset.from,part)&&same(line.dataset.to,path[i+1]))||(same(line.dataset.to,part)&&same(line.dataset.from,path[i+1])))`)
	script = strings.ReplaceAll(script, `x.line.setAttribute('x2',b.x);x.line.setAttribute('y2',b.y)`, `const dx=b.x-a.x,dy=b.y-a.y,len=Math.hypot(dx,dy)||1,pad=Math.max(30,Math.min(82,Math.abs(dx)/len*82+Math.abs(dy)/len*30));x.line.setAttribute('x2',b.x-dx/len*pad);x.line.setAttribute('y2',b.y-dy/len*pad)`)
	script = strings.ReplaceAll(script, `result.textContent=data.allowed?'✓ 許可':'✕ 拒否';`, `window.FGA_CHECK_PATHS=data.allowed?(data.paths||[]):[];window.fgalensHighlightPaths(window.FGA_CHECK_PATHS);if(!data.allowed)window.fgalensHighlightRegions(data.regions||null);result.textContent=data.allowed?'✓ 許可':'✕ 拒否';if(data.allowed&&(data.paths||[]).length){data.paths.forEach(path=>{const line=document.createElement('div');line.textContent=path;line.style.marginTop='4px';result.append(line)})}`)
	script = strings.ReplaceAll(script, `result.textContent=data.allowed?'✓ 許可':'✕ 拒否';`, `result.textContent=data.allowed?(window.FGA_LANG==='en'?'✓ Allowed':'✓ 許可'):(window.FGA_LANG==='en'?'✕ Denied':'✕ 拒否');`)
	script = strings.ReplaceAll(script, `let mode='tuple',zoom=1`, `let mode='tuple',layout='circle',zoom=1`)
	// Expand userset references so the authorization path is visible as user → group → userset → resource.
	script = strings.ReplaceAll(script, `if(mode==='tuple'||mode==='all')tuples.forEach(t=>{add(t.user,'principal');add(t.object,'resource');edges.push({a:t.user,b:t.object,label:t.relation})});`, `if(mode==='tuple'||mode==='all')tuples.forEach(t=>{const i=t.user.indexOf('#'),set=i>0?{object:t.user.slice(0,i),relation:t.user.slice(i+1)}:null;add(t.user,set?'userset':'principal');add(t.object,'resource');if(set){add(set.object,'resource');edges.push({a:set.object,b:t.user,label:set.relation+' userset'})}edges.push({a:t.user,b:t.object,label:t.relation})});`)
	script = strings.ReplaceAll(script, `const data=makeGraph();if(mode==='tuple')data.nodes.forEach(n=>{n.x=n.kind==='principal'?140:760});`, `const data=makeGraph();if(layout==='left-right')data.nodes.forEach(n=>{n.x=n.kind==='principal'?140:n.kind==='resource'?760:450});if(layout==='circle')data.nodes.forEach((n,i)=>{const a=i/data.nodes.length*Math.PI*2;n.x=450+Math.cos(a)*330;n.y=260+Math.sin(a)*190});if(layout==='grid')data.nodes.forEach((n,i)=>{n.x=140+(i%4)*210;n.y=100+Math.floor(i/4)*110});if(layout==='force'&&mode==='tuple')data.nodes.forEach(n=>{n.x=n.kind==='principal'?140:760});`)
	script = strings.ReplaceAll(script, `simulation=0;update();tick()`, `simulation=0;update();if(layout==='force')tick()`)
	script = strings.ReplaceAll(script, `draw()})()`, `draw();window.fgalensSetLayout=v=>{layout=v;draw()}})()`)

	return template.JS(script) //nolint:gosec // script is built-in code with the color rule replaced.
}

func jsonTemplate(v any) template.JS { b, _ := json.Marshal(v); return template.JS(b) } //nolint:gosec // JSON is embedded as a script literal.

const layoutControlJS = `;(()=>{const label=document.createElement('label');label.textContent='レイアウト';label.style.cssText='display:flex;align-items:center;gap:8px;margin:0 0 12px;color:#71808e;font-size:11px';const select=document.createElement('select');select.innerHTML='<option value="force">力学</option><option value="left-right">左 → 右</option><option value="circle">円形</option><option value="grid">グリッド</option>';select.value=localStorage.getItem('fgalens.layout')||'circle';select.style.cssText='display:inline-block;width:120px;margin:0;padding:6px';select.onchange=()=>{localStorage.setItem('fgalens.layout',select.value);window.fgalensSetLayout(select.value)};label.append(select);document.querySelector('.tabs').before(label)})()`

const checkHighlightJS = `;(()=>{const same=(id,part)=>id===part||id===part.split('#')[0];window.fgalensHighlightPaths=paths=>{const pathList=(paths||[]).map(path=>path.split(' → '));document.querySelectorAll('[data-node]').forEach(node=>{const hot=pathList.some(path=>path.some(part=>same(node.dataset.node,part)));node.querySelector('rect').setAttribute('stroke',hot?'#24a148':'#6f7d87');node.querySelector('rect').setAttribute('stroke-width',hot?'3.5':'2')});document.querySelectorAll('line[data-from]').forEach(line=>{const hot=pathList.some(path=>path.some((part,i)=>i<path.length-1&&((same(line.dataset.from,part)&&same(line.dataset.to,path[i+1]))||(same(line.dataset.to,part)&&same(line.dataset.from,path[i+1])))));line.setAttribute('stroke',hot?'#24a148':'#8b98a1');line.setAttribute('stroke-width',hot?'3.5':'1.8')})}})()`

const checkRegionJS = `;(()=>{window.fgalensHighlightRegions=regions=>{const source=new Set((regions&&regions.source_nodes)||[]),target=new Set((regions&&regions.target_nodes)||[]),color=(id=>source.has(id)&&target.has(id)?'#8d67d8':source.has(id)?'#2f7df6':target.has(id)?'#f08a55':'#6f7d87');document.querySelectorAll('[data-node]').forEach(node=>{const stroke=color(node.dataset.node);node.querySelector('rect').setAttribute('stroke',stroke);node.querySelector('rect').setAttribute('stroke-width',stroke==='#6f7d87'?'2':'3.5')});document.querySelectorAll('line[data-from]').forEach(line=>{const from=line.dataset.from,to=line.dataset.to,sourceEdge=source.has(from)&&source.has(to),targetEdge=target.has(from)&&target.has(to),stroke=sourceEdge&&targetEdge?'#8d67d8':sourceEdge?'#2f7df6':targetEdge?'#f08a55':'#8b98a1';line.setAttribute('stroke',stroke);line.setAttribute('stroke-width',sourceEdge||targetEdge?'3.5':'1.8')})}})()`

const checkJS = `;(()=>{const form=document.querySelector('#filters'),search=form.querySelector('button[type="submit"]'),button=document.createElement('button'),result=document.createElement('div');button.type='button';button.className='button';button.textContent='Check を実行';button.style.width='100%';button.style.marginTop='8px';result.setAttribute('aria-live','polite');result.style.cssText='font-size:12px;margin-top:8px;text-align:center';search.after(button);button.after(result);button.onclick=async()=>{button.disabled=true;result.textContent='確認中…';try{const response=await fetch('/check',{method:'POST',body:new URLSearchParams(new FormData(form))});if(!response.ok)throw new Error(await response.text());const data=await response.json();result.textContent=data.allowed?'✓ 許可':'✕ 拒否';result.style.color=data.allowed?'#27814b':'#b44738'}catch(error){result.textContent='Check に失敗しました';result.style.color='#b44738'}finally{button.disabled=false}}})()`

const checkPanelJS = `;(()=>{const searchForm=document.querySelector('#filters'),oldButton=searchForm.querySelector('button:not([type="submit"])');if(oldButton){oldButton.nextElementSibling?.remove();oldButton.remove()}const panel=document.createElement('form');panel.id='check-form';panel.innerHTML='<b>認可を Check</b><label>User<input name="user" placeholder="user:alice"></label><label>Relation<input name="relation" placeholder="viewer"></label><label>Object<input name="object" placeholder="document:design"></label><button class="button" type="submit" style="width:100%">Check を実行</button><div aria-live="polite" style="font-size:12px;margin-top:8px;text-align:center"></div>';searchForm.after(panel);const result=panel.querySelector('[aria-live]');panel.onsubmit=async event=>{event.preventDefault();const button=panel.querySelector('button');button.disabled=true;result.textContent='確認中…';try{const response=await fetch('/check',{method:'POST',body:new URLSearchParams(new FormData(panel))});if(!response.ok)throw new Error(await response.text());const data=await response.json();result.textContent=data.allowed?'✓ 許可':'✕ 拒否';result.style.color=data.allowed?'#27814b':'#b44738'}catch(error){result.textContent='Check に失敗しました';result.style.color='#b44738'}finally{button.disabled=false}}})()`

const checkPersistJS = `;(()=>{const form=document.querySelector('#check-form');if(!form)return;const key='fgalens.check';try{const saved=JSON.parse(localStorage.getItem(key)||'{}');form.querySelectorAll('input[name]').forEach(input=>{if(saved[input.name])input.value=saved[input.name]})}catch(error){}form.addEventListener('input',()=>{const values={};form.querySelectorAll('input[name]').forEach(input=>{values[input.name]=input.value});localStorage.setItem(key,JSON.stringify(values))})})()`

const tableSortJS = `;(()=>{const table=document.querySelector('.table-card table');if(!table)return;const head=[...table.querySelectorAll('thead th')],body=table.querySelector('tbody');let column=-1,descending=false;head.forEach((th,index)=>{th.style.cursor='pointer';th.title='クリックして並べ替え';th.addEventListener('click',()=>{if(column===index)descending=!descending;else{column=index;descending=false}head.forEach((item,i)=>{item.textContent=['User','Relation','Object'][i]+(i===column?(descending?' ↓':' ↑'):'')});[...body.querySelectorAll('tr')].sort((a,b)=>{const left=a.cells[index]?.textContent.trim()||'',right=b.cells[index]?.textContent.trim()||'';return descending?right.localeCompare(left):left.localeCompare(right)}).forEach(row=>body.append(row))})})})()`

const languageJS = `;(()=>{const key='fgalens.language',translations={ja:{status:'OpenFGA / 最新モデル',title:'認可関係を<br><span>可視化</span>',intro:'OpenFGA Store の tuple と最新 authorization model を読み込みます。',object:'Object',relation:'Relation',user:'User',search:'検索して表示 →',refresh:'自動更新',disabled:'無効',seconds:'秒',flow:'アクセスの流れを見る',tupleTab:'保存 tuple',modelTab:'Model relation',partsTab:'tuple 要素分解',allTab:'統合ビュー',tuples:'取得した tuple',model:'Authorization model',dsl:'Authorization model DSL',reset:'検索をリセット',layout:'レイアウト',check:'認可を Check',checkButton:'Check を実行',switch:'English'},en:{status:'OpenFGA / Latest model',title:'Visualize<br><span>authorization</span>',intro:'Load tuples and the latest authorization model from the OpenFGA Store.',object:'Object',relation:'Relation',user:'User',search:'Search →',refresh:'Auto refresh',disabled:'Disabled',seconds:'sec',flow:'Explore authorization flow',tupleTab:'Stored tuples',modelTab:'Model relations',partsTab:'Tuple parts',allTab:'Combined view',tuples:'Retrieved tuples',model:'Authorization model',dsl:'Authorization model DSL',reset:'Reset search',layout:'Layout',check:'Check authorization',checkButton:'Run Check',switch:'日本語'}};const text=(selector,value)=>{const element=document.querySelector(selector);if(element)element.textContent=value};const setLanguage=language=>{const t=translations[language];document.documentElement.lang=language;text('.status',t.status);document.querySelector('h1').innerHTML=t.title;text('aside>.muted',t.intro);text('.settings b',t.refresh);text('.heading h2',t.flow);text('.table-card .card-head b',t.tuples);text('.model-card .card-head b',t.model);text('.tabs button[data-mode="tuple"]',t.tupleTab);text('.tabs button[data-mode="model"]',t.modelTab);text('.tabs button[data-mode="parts"]',t.partsTab);text('.tabs button[data-mode="all"]',t.allTab);document.querySelectorAll('#filters label').forEach(label=>{const input=label.querySelector('input');if(input){label.childNodes[0].nodeValue=t[input.name].replace(/^./,value=>value.toUpperCase())}});text('#filters button[type="submit"]',t.search);document.querySelectorAll('#refresh option').forEach(option=>{if(option.value==='0')option.textContent=t.disabled;else option.textContent=option.value+' '+t.seconds});const reset=document.querySelector('#filters a');if(reset)reset.textContent=t.reset;const layout=document.querySelector('label[style*="align-items"]');if(layout)layout.childNodes[0].nodeValue=t.layout;const check=document.querySelector('#check-form');if(check){text('#check-form b',t.check);text('#check-form button',t.checkButton);check.querySelectorAll('label').forEach(label=>{const input=label.querySelector('input');if(input)label.childNodes[0].nodeValue=t[input.name].replace(/^./,value=>value.toUpperCase())})}const dslCard=[...document.querySelectorAll('.card-head b')].find(element=>element.textContent.includes('DSL')||element.textContent.includes('DSL'));if(dslCard)dslCard.textContent=t.dsl;const toggle=document.querySelector('#language-toggle');if(toggle)toggle.textContent=t.switch;localStorage.setItem(key,language)};const toggle=document.createElement('button');toggle.id='language-toggle';toggle.className='button';toggle.type='button';toggle.style.marginLeft='8px';toggle.onclick=()=>setLanguage((localStorage.getItem(key)||'ja')==='ja'?'en':'ja');const refreshLink=document.querySelector('header .button.dark');refreshLink.before(toggle);setLanguage(localStorage.getItem(key)||'ja')})()`

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()

		next.ServeHTTP(w, r)
		log.Printf("request completed in %s", time.Since(started))
	})
}

const legendHTML = `<div class="legend"><b>表示モード</b><span><i class="blue"></i>tuple の主体 → 対象</span><span><i class="orange"></i>対象間の継承</span><span><i class="purple"></i>要素分解</span></div>`

const pageTemplateText = `{{define "page"}}
<!doctype html><html lang="ja"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>FGA Lens</title><style>{{.CSS}}</style></head><body>
<header><div class="brand">✳ <strong>FGA LENS</strong></div><div class="status">OpenFGA / 最新モデル</div><a class="button dark" href="{{.RefreshURL}}">↻ Refresh</a></header>
<main><aside><div class="eyebrow">OPENFGA TUPLES</div><h1>認可関係を<br><span>可視化</span></h1><p class="muted">OpenFGA Store の tuple と最新 authorization model を読み込みます。</p><form method="get" action="/" id="filters"><label>Object<input name="object" value="{{.Object}}" placeholder="document:report"></label><label>Relation<input name="relation" value="{{.Relation}}" placeholder="viewer"></label><label>User<input name="user" value="{{.User}}" placeholder="user:alice / group:admins#member"></label><button class="button primary" type="submit">検索して表示 →</button></form><div class="settings"><b>自動更新</b><select id="refresh"><option value="0">無効</option><option value="10">10秒</option><option value="30" selected>30秒</option><option value="60">60秒</option></select></div><div class="legend"><b>表示モード</b><span><i class="blue"></i>tuple の主体 → 対象</span><span><i class="orange"></i>対象間の継承</span><span><i class="purple"></i>要素分解</span></div></aside><section class="content"><div class="heading"><div><div class="eyebrow">VISUAL MAP / RELATIONSHIP GRAPH</div><h2>アクセスの流れを見る</h2></div><div class="exports"><button class="button" id="svg">SVG</button><button class="button" id="png">PNG</button></div></div>{{if .Error}}<div class="error">{{.Error}}<span>前回の結果がある場合は、そのまま保持されます。</span></div>{{end}}<div class="stats"><div><small>TUPLES</small><strong>{{len .Tuples}}</strong></div><div><small>MODEL TYPES</small><strong>{{len .Model.Types}}</strong></div><div><small>MODEL RELATIONS</small><strong>{{len .Model.Relations}}</strong></div></div><div class="tabs"><button data-mode="tuple" class="active">主体 → 対象</button><button data-mode="model">継承モデル</button><button data-mode="parts">要素分解</button><button data-mode="all">統合ビュー</button></div><div class="card"><div class="card-head"><b id="summary">{{len .Tuples}} 件の tuple</b><span>更新: {{.UpdatedAt}}</span></div><div id="graph" class="graph"><div class="empty" id="empty">データがありません</div></div><div class="card-foot">モデル: {{.Model.ID}} · schema {{.Model.Schema}} · 最大 {{.PageSize}} 件</div></div><div class="below"><div class="card table-card"><div class="card-head"><b>取得した tuple</b>{{if .ContinuationToken}}<a href="{{.NextURL}}">次の {{.PageSize}} 件 →</a>{{end}}</div><table><thead><tr><th>User</th><th>Relation</th><th>Object</th></tr></thead><tbody>{{range .Tuples}}<tr><td>{{.User}}</td><td class="relation">{{.Relation}}</td><td>{{.Object}}</td></tr>{{else}}<tr><td colspan="3" class="muted">tuple がありません</td></tr>{{end}}</tbody></table></div><div class="card model-card"><div class="card-head"><b>Authorization model</b></div>{{range .Model.Relations}}<div class="model-row"><code>{{.Type}}</code><span>{{.Name}}</span><small>{{.Expression}}</small></div>{{end}}</div></div></section></main>
<script>window.FGA={{js .TuplesJSON}};window.MODEL={{js .ModelJSON}};</script><script>{{.JS}}</script></body></html>{{end}}`

const pageCSS = `:root{--ink:#17202b;--muted:#71808e;--line:#dce4e9;--paper:#f6f9fb;--blue:#2f7df6;--orange:#f08a55;--purple:#8d67d8;--navy:#142334}*{box-sizing:border-box}body{margin:0;background:var(--paper);color:var(--ink);font:13px system-ui,sans-serif}header{height:68px;background:#fff;border-bottom:1px solid var(--line);display:flex;align-items:center;justify-content:space-between;padding:0 30px}.brand{font-size:17px;letter-spacing:.12em;color:var(--blue)}.brand strong{color:var(--ink)}.status,.muted{color:var(--muted)}main{display:grid;grid-template-columns:300px 1fr;min-height:calc(100vh - 68px)}aside{background:#fff;border-right:1px solid var(--line);padding:32px 25px}.eyebrow,small{font-size:10px;letter-spacing:.14em;font-weight:800;color:#8c9aa6}h1{font-size:28px;line-height:1.15;margin:25px 0 12px}h1 span{color:var(--blue)}h2{font-size:29px;margin:10px 0 0}.muted{line-height:1.6}.content{padding:31px 35px;max-width:1400px;width:100%;margin:auto}.heading{display:flex;justify-content:space-between;align-items:end;margin-bottom:22px}form{margin-top:25px}label{display:block;color:#586a77;font-weight:700;font-size:11px;margin:14px 0}input,select{display:block;width:100%;border:1px solid #cdd8e0;border-radius:5px;padding:10px;margin-top:6px;font:12px monospace;color:var(--ink);background:#fff}.button{border:1px solid var(--line);background:#fff;border-radius:5px;padding:9px 13px;color:#657582;cursor:pointer;text-decoration:none;font-weight:700}.button.dark{background:var(--navy);border-color:var(--navy);color:#fff}.button.primary{width:100%;background:var(--blue);border-color:var(--blue);color:#fff;margin-top:10px}.settings{display:flex;align-items:center;justify-content:space-between;border-top:1px solid var(--line);margin-top:28px;padding-top:20px}.settings select{width:100px;margin:0}.legend{border-top:1px solid var(--line);margin-top:25px;padding-top:20px;color:var(--muted)}.legend b,.legend span{display:block;margin:11px 0}.legend i{display:inline-block;width:20px;border-top:3px solid;margin-right:8px}.legend i.orange{border-color:var(--orange)}.legend i.purple{border-color:var(--purple)}.stats{display:grid;grid-template-columns:repeat(3,1fr);gap:12px;margin-bottom:15px}.stats>div,.card{background:#fff;border:1px solid var(--line);border-radius:8px}.stats>div{padding:15px 17px}.stats strong{display:block;font:24px monospace;margin-top:6px}.tabs{display:flex;gap:4px;margin-bottom:10px}.tabs button{border:1px solid var(--line);background:#fff;padding:9px 13px;border-radius:5px;color:#667783;cursor:pointer}.tabs button.active{background:var(--navy);color:#fff}.card-head{height:50px;border-bottom:1px solid var(--line);display:flex;align-items:center;justify-content:space-between;padding:0 17px}.card-head span{font-size:11px;color:var(--muted)}.graph{height:440px;position:relative;overflow:hidden;background-image:radial-gradient(#d8e1e6 1px,transparent 1px);background-size:22px 22px}.graph svg{width:100%;height:100%}.empty{position:absolute;inset:0;display:grid;place-items:center;color:var(--muted)}.card-foot{border-top:1px solid var(--line);padding:10px 17px;color:#87949e;font-size:11px}.below{display:grid;grid-template-columns:1.2fr .8fr;gap:13px;margin-top:13px}table{border-collapse:collapse;width:100%;font:12px monospace}th,td{text-align:left;padding:11px 17px;border-bottom:1px solid #edf1f3}th{color:#87949e;font:10px system-ui;letter-spacing:.1em}.relation{color:var(--blue)}.model-row{display:grid;grid-template-columns:1fr 1fr;gap:5px;padding:10px 17px;border-bottom:1px solid #edf1f3}.model-row code,.model-row small{font-family:monospace}.model-row small{grid-column:1/-1;color:var(--muted);font-size:10px}.error{background:#fff0ed;border:1px solid #f2b4a6;color:#a33d2b;padding:12px 15px;border-radius:6px;margin-bottom:15px}.error span{display:block;color:#8b6b66;font-size:11px;margin-top:5px}@media(max-width:900px){main{display:block}aside{border:0;border-bottom:1px solid var(--line)}.content{padding:24px 15px}.below{grid-template-columns:1fr}}@media(max-width:540px){header{padding:0 15px}.status{display:none}.heading{display:block}.exports{margin-top:15px}.stats{grid-template-columns:1fr}.tabs{overflow:auto}.graph{height:380px}}`

const pageJS = `(()=>{const graph=document.querySelector('#graph'),refresh=document.querySelector('#refresh'),form=document.querySelector('#filters');let timer;function esc(s){return String(s).replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]))}function draw(mode='tuple'){const tuples=window.FGA||[],model=window.MODEL||{},nodes=new Map(),edges=[];function node(id,kind){if(!nodes.has(id))nodes.set(id,{id,kind})}if(mode==='tuple'||mode==='all'){tuples.forEach(t=>{node(t.user,'user');node(t.object,'object');edges.push({a:t.user,b:t.object,label:t.relation,c:'blue'})})}if(mode==='model'||mode==='all'){(model.relations||[]).forEach((r,i)=>{const a=r.type+':'+r.name;node(a,'relation');if(r.expression.indexOf('from:')===0){node(r.type+':'+r.expression.slice(6).split(' → ')[0],'relation');edges.push({a,b:r.type+':'+r.expression.slice(6).split(' → ')[0],label:'継承',c:'orange'})}else if(r.expression.indexOf('computed:')===0){node(r.type+':'+r.expression.slice(10),'relation');edges.push({a,b:r.type+':'+r.expression.slice(10),label:'computed',c:'purple'})}})}if(mode==='parts'){tuples.forEach(t=>{const a='user='+t.user,b='relation='+t.relation,c='object='+t.object;node(a,'user');node(b,'relation');node(c,'object');edges.push({a,b,label:'',c:'purple'},{a:b,b:c,label:'',c:'purple'})})}if(!nodes.size){graph.innerHTML='<div class="empty">表示するデータがありません</div>';return}const arr=[...nodes.values()],w=graph.clientWidth||700,h=440,cx=w/2,cy=h/2,rx=Math.max(120,w*.35),ry=145;arr.forEach((n,i)=>{const a=arr.length===1?-Math.PI/2:i/arr.length*Math.PI*2-Math.PI/2;n.x=cx+Math.cos(a)*rx;n.y=cy+Math.sin(a)*ry});let svg='<svg viewBox="0 0 '+w+' '+h+'" xmlns="http://www.w3.org/2000/svg"><defs><marker id="arrow" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto"><path d="M0 0L8 4L0 8z" fill="#2f7df6"/></marker></defs>';const by=new Map(arr.map(n=>[n.id,n]));edges.forEach(e=>{const a=by.get(e.a),b=by.get(e.b);if(!a||!b)return;svg+='<path d="M'+a.x+' '+a.y+' Q '+((a.x+b.x)/2)+' '+((a.y+b.y)/2-25)+' '+b.x+' '+b.y+'" fill="none" stroke="var(--'+e.c+')" stroke-width="2" marker-end="url(#arrow)"/><text x="'+((a.x+b.x)/2)+'" y="'+((a.y+b.y)/2-30)+'" text-anchor="middle" fill="#60717d" font-size="11">'+esc(e.label)+'</text>'});arr.forEach(n=>{const color=n.kind==='user'?'#2f7df6':n.kind==='object'?'#f08a55':'#8d67d8';svg+='<g><rect x="'+(n.x-70)+'" y="'+(n.y-22)+'" width="140" height="44" rx="6" fill="white" stroke="'+color+'" stroke-width="2"/><text x="'+n.x+'" y="'+(n.y+5)+'" text-anchor="middle" font-family="monospace" font-size="11">'+esc(n.id)+'</text></g>'});graph.innerHTML=svg+'</svg>'}document.querySelectorAll('.tabs button').forEach(b=>b.onclick=()=>{document.querySelectorAll('.tabs button').forEach(x=>x.classList.remove('active'));b.classList.add('active');draw(b.dataset.mode)});function setTimer(){clearInterval(timer);const seconds=Number(refresh.value);if(seconds)timer=setInterval(()=>form.submit(),seconds*1000)}refresh.onchange=setTimer;draw();setTimer();function download(type){const svg=graph.querySelector('svg');if(!svg)return;const data=new XMLSerializer().serializeToString(svg);if(type==='svg'){const a=document.createElement('a');a.href='data:image/svg+xml;charset=utf-8,'+encodeURIComponent(data);a.download='fga-lens.svg';a.click()}else{const image=new Image;image.onload=()=>{const c=document.createElement('canvas');c.width=svg.viewBox.baseVal.width*2;c.height=svg.viewBox.baseVal.height*2;c.getContext('2d').drawImage(image,0,0,c.width,c.height);const a=document.createElement('a');a.href=c.toDataURL('image/png');a.download='fga-lens.png';a.click()};image.src='data:image/svg+xml;charset=utf-8,'+encodeURIComponent(data)}}document.querySelector('#svg').onclick=()=>download('svg');document.querySelector('#png').onclick=()=>download('png')})()`

const dynamicJS = `;(()=>{const host=document.querySelector('#graph'),W=900,H=520;let mode='tuple',zoom=1,pan={x:0,y:0},drag=null,simulation=0;const esc=s=>String(s).replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));function makeGraph(){const tuples=window.FGA||[],model=window.MODEL||{},nodes=new Map(),edges=[];const add=(id,kind)=>{if(!nodes.has(id))nodes.set(id,{id,kind,x:W/2+(Math.random()-.5)*500,y:H/2+(Math.random()-.5)*260,vx:0,vy:0})};if(mode==='tuple'||mode==='all')tuples.forEach(t=>{add(t.user,'principal');add(t.object,'resource');edges.push({a:t.user,b:t.object,label:t.relation})});if(mode==='model'||mode==='all')(model.relations||[]).forEach(r=>{const a=r.type+':'+r.name;add(a,'relation');if((r.expression||'').indexOf('from:')===0){const b=r.type+':'+r.expression.slice(6).split(' → ')[0];add(b,'relation');edges.push({a,b,label:'継承'})}else if((r.expression||'').indexOf('computed:')===0){const b=r.type+':'+r.expression.slice(10);add(b,'relation');edges.push({a,b,label:'computed'})}});if(mode==='parts')tuples.forEach(t=>{const a='user='+t.user,b='relation='+t.relation,c='object='+t.object;add(a,'part');add(b,'part');add(c,'part');edges.push({a,b,label:''},{a:b,b:c,label:''})});return {nodes:[...nodes.values()],edges}}function draw(){const data=makeGraph();host.innerHTML='<svg viewBox="0 0 '+W+' '+H+'" aria-label="Interactive OpenFGA graph"><g id="viewport"></g></svg>';const svg=host.querySelector('svg'),view=host.querySelector('#viewport'),by=new Map(data.nodes.map(n=>[n.id,n])),lines=data.edges.map(e=>{const line=document.createElementNS('http://www.w3.org/2000/svg','line');line.setAttribute('stroke','#8b98a1');line.setAttribute('stroke-width','1.8');view.append(line);return {e,line}}),labels=data.edges.map(e=>{const text=document.createElementNS('http://www.w3.org/2000/svg','text');text.textContent=e.label;text.setAttribute('text-anchor','middle');text.setAttribute('fill','#60717d');text.setAttribute('font-size','11');view.append(text);return {e,text}}),nodeEls=data.nodes.map(n=>{const g=document.createElementNS('http://www.w3.org/2000/svg','g');g.classList.add('interactive-node');g.innerHTML='<rect x="-76" y="-24" width="152" height="48" rx="7" fill="#fff" stroke="#6f7d87" stroke-width="2"/><text x="0" y="5" text-anchor="middle" font-family="monospace" font-size="11">'+esc(n.id)+'</text>';g.addEventListener('pointerdown',e=>{e.stopPropagation();drag={node:n};g.setPointerCapture(e.pointerId)});g.addEventListener('pointermove',e=>{if(!drag||drag.node!==n)return;const r=svg.getBoundingClientRect();n.x=((e.clientX-r.left)/r.width*W-pan.x)/zoom;n.y=((e.clientY-r.top)/r.height*H-pan.y)/zoom;n.vx=n.vy=0;update()});g.addEventListener('pointerup',()=>{if(drag&&drag.node===n)drag=null});g.addEventListener('mouseenter',()=>g.classList.add('hot'));g.addEventListener('mouseleave',()=>g.classList.remove('hot'));g.setAttribute('transform','translate('+n.x+' '+n.y+')');view.append(g);return {n,g}});function update(){view.setAttribute('transform','translate('+pan.x+' '+pan.y+') scale('+zoom+')');nodeEls.forEach(x=>x.g.setAttribute('transform','translate('+x.n.x+' '+x.n.y+')'));lines.forEach(x=>{const a=by.get(x.e.a),b=by.get(x.e.b);if(a&&b){x.line.setAttribute('x1',a.x);x.line.setAttribute('y1',a.y);x.line.setAttribute('x2',b.x);x.line.setAttribute('y2',b.y)}});labels.forEach(x=>{const a=by.get(x.e.a),b=by.get(x.e.b);if(a&&b){x.text.setAttribute('x',(a.x+b.x)/2);x.text.setAttribute('y',(a.y+b.y)/2-8)}})}function tick(){let moving=false;data.nodes.forEach(n=>{n.vx+=(W/2-n.x)*.002;n.vy+=(H/2-n.y)*.002;data.nodes.forEach(m=>{if(n===m)return;const dx=n.x-m.x,dy=n.y-m.y,d2=dx*dx+dy*dy+80,norm=Math.sqrt(d2);n.vx+=dx/d2*180;n.vy+=dy/d2*180});n.vx*=.82;n.vy*=.82;n.x+=n.vx;n.y+=n.vy;n.x=Math.max(80,Math.min(W-80,n.x));n.y=Math.max(35,Math.min(H-35,n.y));if(Math.abs(n.vx)+Math.abs(n.vy)>.2)moving=true});update();if(moving&&simulation++<180)requestAnimationFrame(tick)}svg.addEventListener('pointerdown',e=>{if(e.target!==svg)return;drag={pan:{x:e.clientX,y:e.clientY},origin:{...pan}};svg.setPointerCapture(e.pointerId)});svg.addEventListener('pointermove',e=>{if(!drag||!drag.pan)return;pan.x=drag.origin.x+e.clientX-drag.pan.x;pan.y=drag.origin.y+e.clientY-drag.pan.y;update()});svg.addEventListener('pointerup',()=>drag=null);svg.addEventListener('wheel',e=>{e.preventDefault();zoom=Math.max(.45,Math.min(2.5,zoom+(e.deltaY<0?.1:-.1)));update()},{passive:false});svg.addEventListener('dblclick',()=>{pan={x:0,y:0};zoom=1;simulation=0;data.nodes.forEach(n=>{n.x=W/2+(Math.random()-.5)*500;n.y=H/2+(Math.random()-.5)*260});tick()});simulation=0;update();tick()}document.querySelectorAll('.tabs button').forEach(b=>b.addEventListener('click',()=>{mode=b.dataset.mode;draw()}));draw()})()`

const pagePersistJS = `;(()=>{const key='fgalens.viewMode',buttons=document.querySelectorAll('.tabs button'),form=document.querySelector('#filters'),refresh=document.querySelector('#refresh');refresh.value='0';refresh.dispatchEvent(new Event('change'));const order=['user','relation','object'];[...form.querySelectorAll('label')].sort((a,b)=>order.indexOf(a.querySelector('input').name)-order.indexOf(b.querySelector('input').name)).forEach(label=>form.insertBefore(label,form.querySelector('button')));buttons.forEach(b=>b.addEventListener('click',()=>localStorage.setItem(key,b.dataset.mode)));const saved=localStorage.getItem(key);const button=[...buttons].find(b=>b.dataset.mode===saved);if(button&&!button.classList.contains('active'))button.click();const reset=document.createElement('a');reset.href='/';reset.className='button';reset.textContent='検索をリセット';reset.style.display='block';reset.style.textAlign='center';reset.style.marginTop='8px';form.append(reset);const dsl=document.createElement('div');dsl.className='card';dsl.style.marginTop='13px';dsl.innerHTML='<div class="card-head"><b>Authorization model DSL</b></div><pre style="margin:0;padding:17px;overflow:auto;font:12px/1.7 monospace;background:#132333;color:#d4e3ef">'+esc((window.MODEL&&window.MODEL.dsl)||'モデル DSL はありません')+'</pre>';document.querySelector('.content').append(dsl)})()`
