# Agent Guidelines for KubeTag

## Go Backend Rules

### Code Organization
- Follow clean architecture principles with clear separation of concerns
- Structure: `cmd/`, `internal/`, `pkg/` directories
- Keep handlers thin, business logic in services
- Use interfaces for testability and dependency injection

### Style & Conventions
- Follow official Go style guide and `gofmt` formatting
- Use meaningful variable names, avoid single letters except for loops
- Error handling: always check errors, wrap with context using `fmt.Errorf`
- Use `context.Context` for cancellation and timeouts
- Prefer concrete types for return values, interfaces for parameters

### Kubernetes Client
- Use official `k8s.io/client-go` library
- Implement graceful error handling for cluster connectivity issues
- Support both in-cluster and out-of-cluster configurations
- Use informers for watching resources efficiently

### Fiber Framework
- Use Fiber v2 for REST endpoints
- Group related routes using `fiber.Router`
- Implement proper middleware (logging, CORS, recovery)
- Use structured response formats
- Enable request validation

### Dependencies
- Use Go modules for dependency management
- Pin major versions, allow minor/patch updates
- Minimize external dependencies
- Prefer standard library when possible

### Testing
- Write unit tests for business logic
- Use table-driven tests
- Mock external dependencies (Kubernetes API)
- Target 70%+ code coverage

## Frontend Rules (ShadCN UI - No React)

### Technology Stack
- Pure HTML, CSS, and vanilla JavaScript
- Use ShadCN UI components (adapted for vanilla JS)
- Tailwind CSS for styling
- No build step required - keep it simple

### Code Organization
- Single `index.html` file for simplicity
- Inline CSS and JS or use separate files if needed
- Keep JavaScript modular with clear functions
- Use modern ES6+ syntax

### UI/UX Guidelines
- Clean, minimal interface
- Responsive design (mobile-first)
- Dark mode support
- Accessible components (ARIA labels, keyboard navigation)
- Loading states and error handling

### API Integration
- Use `fetch` API for backend communication
- Implement proper error handling and retries
- Show loading spinners during requests
- Display user-friendly error messages

### State Management
- Keep state simple with vanilla JS
- Use local storage for user preferences
- Implement efficient DOM updates

### Performance
- Minimize DOM manipulations
- Use event delegation where appropriate
- Lazy load images if needed
- Keep bundle size small

## General Principles

### Security
- Validate all inputs
- Sanitize user data
- Use RBAC for Kubernetes access
- No hardcoded credentials
- CORS configuration for API

### Documentation
- Clear code comments for complex logic
- API documentation for all endpoints
- README with quick start guide
- Architecture decisions documented

### Git Practices
- Clear, descriptive commit messages
- Small, focused commits
- Feature branches for new work
- No secrets in repository
