// Command sample demonstrates the github.com/bjaus/api framework
// with a realistic API covering every major feature.
//
// Run:
//
//	go run ./cmd/sample
//
// Generate the OpenAPI spec:
//
//	go run ./cmd/sample -spec                        — print to stdout
//	go run ./cmd/sample -spec -o openapi.json        — write to file
//	go run ./cmd/sample -spec | pbcopy               — copy to clipboard (macOS)
//
// Then explore:
//
//	GET  http://localhost:8080/openapi.json          — OpenAPI spec
//	GET  http://localhost:8080/v1/health              — health check
//	GET  http://localhost:8080/v1/users               — list users
//	POST http://localhost:8080/v1/users               — create user
//	GET  http://localhost:8080/v1/users/{id}          — get user
//	PUT  http://localhost:8080/v1/users/{id}          — update user
//	DELETE http://localhost:8080/v1/users/{id}        — delete user
//	POST http://localhost:8080/v1/users/{id}/avatar   — upload avatar
//	GET  http://localhost:8080/v1/users/{id}/avatar   — download avatar
//	GET  http://localhost:8080/v1/events              — SSE event stream
//	GET  http://localhost:8080/v1/ws                  — raw handler (WebSocket placeholder)
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"

	"github.com/bjaus/api"
)

func main() {
	specFlag := flag.Bool("spec", false, "Print the OpenAPI spec to stdout and exit")
	outFlag := flag.String("o", "", "Output file for the spec (requires -spec)")
	flag.Parse()

	r := newRouter()

	if *specFlag {
		if err := writeSpec(r, *outFlag); err != nil {
			slog.Error("spec generation failed", "err", err)
			os.Exit(1)
		}
		return
	}

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	slog.Info("starting server", "addr", ":8080", "spec", "http://localhost:8080/openapi.json")

	if err := r.ListenAndServe(ctx, ":8080"); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("server error", "err", err)
	}

	slog.Info("server stopped")
}

func newRouter() *api.Router {
	r := api.New(
		api.WithTitle("Sample API"),
		api.WithVersion("1.0.0"),
		api.WithValidator(&bodyLengthValidator{maxBytes: 1 << 20}), // 1 MB
	)

	// Global middleware.
	r.Use(api.Recovery())
	r.Use(requestLogger())
	r.Use(cors())

	// Serve the OpenAPI spec at the root level.
	r.ServeSpec("/openapi.json")

	// ---------- v1 group ----------

	v1 := r.Group("/v1", api.WithGroupTags("v1"))

	// Health.
	api.Get(v1, "/health", handleHealth,
		api.WithSummary("Health check"),
		api.WithDescription("Returns the current server time and status."),
		api.WithTags("ops"),
	)

	// Users CRUD.
	api.Get(v1, "/users", handleListUsers,
		api.WithSummary("List users"),
		api.WithDescription("Returns all users, with optional filtering by role."),
		api.WithTags("users"),
	)
	api.Post(v1, "/users", handleCreateUser,
		api.WithStatus(http.StatusCreated),
		api.WithSummary("Create user"),
		api.WithTags("users"),
	)
	api.Get(v1, "/users/{id}", handleGetUser,
		api.WithSummary("Get user by ID"),
		api.WithTags("users"),
	)
	api.Put(v1, "/users/{id}", handleUpdateUser,
		api.WithSummary("Update user"),
		api.WithTags("users"),
	)
	api.Delete(v1, "/users/{id}", handleDeleteUser,
		api.WithSummary("Delete user"),
		api.WithTags("users"),
	)

	// File upload / download (avatar).
	api.Post(v1, "/users/{id}/avatar", handleUploadAvatar,
		api.WithStatus(http.StatusNoContent),
		api.WithSummary("Upload avatar"),
		api.WithDescription("Accepts a multipart file upload for the user's avatar."),
		api.WithTags("users", "files"),
	)
	api.Get(v1, "/users/{id}/avatar", handleDownloadAvatar,
		api.WithSummary("Download avatar"),
		api.WithDescription("Returns the user's avatar as a binary stream."),
		api.WithTags("users", "files"),
	)

	// SSE event stream.
	api.Get(v1, "/events", handleEvents,
		api.WithSummary("Event stream"),
		api.WithDescription("Server-Sent Events stream that emits a tick every second."),
		api.WithTags("streaming"),
	)

	// Raw handler escape hatch (e.g. WebSocket placeholder).
	api.Raw(r, http.MethodGet, "/v1/ws", handleWebSocket, api.OperationInfo{
		Summary:     "WebSocket endpoint",
		Description: "Placeholder for a WebSocket upgrade. Demonstrates the Raw handler escape hatch.",
		Tags:        []string{"v1", "streaming"},
		Status:      http.StatusSwitchingProtocols,
	})

	// Deprecated endpoint.
	api.Get(v1, "/legacy", handleLegacy,
		api.WithSummary("Legacy endpoint"),
		api.WithDeprecated(),
		api.WithTags("ops"),
	)

	return r
}

// ---------------------------------------------------------------------------
func writeSpec(r *api.Router, outFile string) error {
	w := os.Stdout
	if outFile != "" {
		f, err := os.Create(outFile) //nolint:gosec // user-provided CLI flag
		if err != nil {
			return err
		}
		defer func() {
			if err := f.Close(); err != nil {
				slog.Error("failed to close output file", "err", err)
			}
		}()
		w = f
	}
	return r.WriteSpec(w)
}

// In-memory store
// ---------------------------------------------------------------------------

var store = &userStore{
	users: map[string]*User{
		"1": {ID: "1", Name: "Alice", Email: "alice@example.com", Role: "admin", CreatedAt: time.Now()},
		"2": {ID: "2", Name: "Bob", Email: "bob@example.com", Role: "member", CreatedAt: time.Now()},
	},
	avatars: map[string][]byte{},
	nextID:  3,
}

type userStore struct {
	mu      sync.RWMutex
	users   map[string]*User
	avatars map[string][]byte
	nextID  int
}

func (s *userStore) list(role string) []User {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]User, 0, len(s.users))
	for _, u := range s.users {
		if role != "" && u.Role != role {
			continue
		}
		out = append(out, *u)
	}
	return out
}

func (s *userStore) get(id string) (*User, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.users[id]
	if !ok {
		return nil, false
	}
	cp := *u
	return &cp, true
}

func (s *userStore) create(name, email, role string) *User {
	s.mu.Lock()
	defer s.mu.Unlock()
	u := &User{
		ID:        fmt.Sprintf("%d", s.nextID),
		Name:      name,
		Email:     email,
		Role:      role,
		CreatedAt: time.Now(),
	}
	s.nextID++
	s.users[u.ID] = u
	cp := *u
	return &cp
}

func (s *userStore) update(id, name, email, role string) (*User, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.users[id]
	if !ok {
		return nil, false
	}
	if name != "" {
		u.Name = name
	}
	if email != "" {
		u.Email = email
	}
	if role != "" {
		u.Role = role
	}
	cp := *u
	return &cp, true
}

func (s *userStore) delete(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.users[id]; !ok {
		return false
	}
	delete(s.users, id)
	delete(s.avatars, id)
	return true
}

func (s *userStore) setAvatar(id string, data []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.avatars[id] = data
}

func (s *userStore) getAvatar(id string) ([]byte, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	data, ok := s.avatars[id]
	return data, ok
}

// ---------------------------------------------------------------------------
// Domain types
// ---------------------------------------------------------------------------

// User is the core domain entity.
type User struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

// ---------------------------------------------------------------------------
// Request / Response types
// ---------------------------------------------------------------------------

// --- Health ---

type HealthResp struct {
	Status string    `json:"status"`
	Time   time.Time `json:"time"`
}

// --- List Users ---

type ListUsersReq struct {
	Role   string `query:"role" doc:"Filter by role (admin, member)" default:""`
	Limit  int    `query:"limit" doc:"Max results" default:"50"`
	Offset int    `query:"offset" doc:"Pagination offset" default:"0"`
}

type ListUsersResp struct {
	Users []User `json:"users"`
	Total int    `json:"total"`
}

// --- Create User ---

type CreateUserReq struct {
	Body struct {
		Name  string `json:"name" required:"true" doc:"Display name"`
		Email string `json:"email" required:"true" doc:"Email address"`
		Role  string `json:"role" doc:"User role (admin, member)" default:"member"`
	}
}

func (r *CreateUserReq) Validate() error {
	if strings.TrimSpace(r.Body.Name) == "" {
		return api.Error(http.StatusBadRequest, "name is required")
	}
	if strings.TrimSpace(r.Body.Email) == "" {
		return api.Error(http.StatusBadRequest, "email is required")
	}
	if !strings.Contains(r.Body.Email, "@") {
		return api.Error(http.StatusBadRequest, "email must contain @")
	}
	return nil
}

// --- Get / Update / Delete User ---

type UserByIDReq struct {
	ID string `path:"id" doc:"User ID"`
}

type UpdateUserReq struct {
	ID   string `path:"id" doc:"User ID"`
	Body struct {
		Name  string `json:"name" doc:"Display name"`
		Email string `json:"email" doc:"Email address"`
		Role  string `json:"role" doc:"User role"`
	}
}

// --- Avatar Upload (uses RawRequest for multipart) ---

type UploadAvatarReq struct {
	api.RawRequest
	ID string `path:"id" doc:"User ID"`
}

// --- Avatar Download ---

type DownloadAvatarReq struct {
	ID string `path:"id" doc:"User ID"`
}

// --- Legacy ---

type LegacyResp struct {
	Message string `json:"message"`
}

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

func handleHealth(_ context.Context, _ *api.Void) (*HealthResp, error) {
	return &HealthResp{
		Status: "ok",
		Time:   time.Now(),
	}, nil
}

func handleListUsers(_ context.Context, req *ListUsersReq) (*ListUsersResp, error) {
	users := store.list(req.Role)
	total := len(users)

	// Apply offset/limit.
	if req.Offset > len(users) {
		users = nil
	} else {
		users = users[req.Offset:]
	}
	if req.Limit > 0 && req.Limit < len(users) {
		users = users[:req.Limit]
	}

	return &ListUsersResp{
		Users: users,
		Total: total,
	}, nil
}

func handleCreateUser(_ context.Context, req *CreateUserReq) (*User, error) {
	role := req.Body.Role
	if role == "" {
		role = "member"
	}
	user := store.create(req.Body.Name, req.Body.Email, role)
	return user, nil
}

func handleGetUser(_ context.Context, req *UserByIDReq) (*User, error) {
	user, ok := store.get(req.ID)
	if !ok {
		return nil, api.Errorf(http.StatusNotFound, "user %s not found", req.ID)
	}
	return user, nil
}

func handleUpdateUser(_ context.Context, req *UpdateUserReq) (*User, error) {
	user, ok := store.update(req.ID, req.Body.Name, req.Body.Email, req.Body.Role)
	if !ok {
		return nil, api.Errorf(http.StatusNotFound, "user %s not found", req.ID)
	}
	return user, nil
}

func handleDeleteUser(_ context.Context, req *UserByIDReq) (*api.Void, error) {
	if !store.delete(req.ID) {
		return nil, api.Errorf(http.StatusNotFound, "user %s not found", req.ID)
	}
	return &api.Void{}, nil
}

func handleUploadAvatar(_ context.Context, req *UploadAvatarReq) (*api.Void, error) {
	if _, ok := store.get(req.ID); !ok {
		return nil, api.Errorf(http.StatusNotFound, "user %s not found", req.ID)
	}

	upload, err := api.ParseFileUpload(req.Request, "avatar")
	if err != nil {
		return nil, api.Errorf(http.StatusBadRequest, "missing avatar file: %v", err)
	}

	rc, err := upload.Open()
	if err != nil {
		return nil, api.Errorf(http.StatusInternalServerError, "failed to read upload: %v", err)
	}
	defer func() {
		//nolint:errcheck,gosec // best-effort close
		rc.Close()
	}()

	buf := make([]byte, upload.Size)
	n, err := rc.Read(buf)
	if err != nil && n == 0 {
		return nil, api.Errorf(http.StatusInternalServerError, "failed to read upload: %v", err)
	}

	store.setAvatar(req.ID, buf[:n])
	return &api.Void{}, nil
}

func handleDownloadAvatar(_ context.Context, req *DownloadAvatarReq) (*api.Stream, error) {
	data, ok := store.getAvatar(req.ID)
	if !ok {
		return nil, api.Errorf(http.StatusNotFound, "avatar not found for user %s", req.ID)
	}

	return &api.Stream{
		ContentType: "image/png",
		Status:      http.StatusOK,
		Body:        strings.NewReader(string(data)),
	}, nil
}

func handleEvents(ctx context.Context, _ *api.Void) (*api.SSEStream, error) {
	ch := make(chan api.SSEEvent)

	go func() {
		defer close(ch)
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()

		i := 0
		for {
			select {
			case <-ctx.Done():
				return
			case t := <-ticker.C:
				i++
				ch <- api.SSEEvent{
					ID:    fmt.Sprintf("%d", i),
					Event: "tick",
					Data:  map[string]any{"time": t.Format(time.RFC3339), "seq": i},
				}
				if i >= 30 {
					return // stop after 30 events for the demo
				}
			}
		}
	}()

	return &api.SSEStream{Events: ch}, nil
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// In a real app you'd upgrade to WebSocket here.
	// This just demonstrates the Raw handler escape hatch.
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	//nolint:errcheck // best-effort response write
	fmt.Fprintln(w, "WebSocket upgrade would happen here.")
	//nolint:errcheck // best-effort response write
	fmt.Fprintf(w, "Method: %s, Path: %s\n", r.Method, r.URL.Path)
}

func handleLegacy(_ context.Context, _ *api.Void) (*LegacyResp, error) {
	return &LegacyResp{Message: "This endpoint is deprecated. Use /v1/health instead."}, nil
}

// ---------------------------------------------------------------------------
// Middleware
// ---------------------------------------------------------------------------

func requestLogger() api.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			next.ServeHTTP(w, r)
			slog.Info("request",
				"method", r.Method,
				"path", r.URL.Path,
				"duration", time.Since(start),
			)
		})
	}
}

func cors() api.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ---------------------------------------------------------------------------
// Global validator
// ---------------------------------------------------------------------------

type bodyLengthValidator struct {
	maxBytes int64
}

func (v *bodyLengthValidator) Validate(req any) error {
	// This demonstrates the global Validator interface.
	// The SelfValidator on CreateUserReq runs first (per-type),
	// then this global validator runs for all requests.
	type withRaw interface{ GetRequest() *http.Request }
	if rr, ok := req.(withRaw); ok {
		r := rr.GetRequest()
		if r != nil && r.ContentLength > v.maxBytes {
			return api.Errorf(http.StatusRequestEntityTooLarge, "body too large: %d > %d", r.ContentLength, v.maxBytes)
		}
	}
	return nil
}
