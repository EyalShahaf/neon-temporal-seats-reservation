# Temporal Seats (Home Assignment)

A small flight seat reservation + payment system orchestrated with **Temporal** (Go SDK).

## What it does
- Users create an order, select seats, and get a **15-minute hold** with visible countdown.
- Changing seats **refreshes** the hold timer.
- Payment is simulated with a **5-digit code**, **10s timeout**, and **3 retries**, with a **15%** random fail rate.
- UI (or `curl`) can subscribe to real-time status via **SSE**.

---

## 🏗️ Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              FRONTEND (React + Vite)                         │
│  ┌────────────┐  ┌─────────────┐  ┌──────────────┐  ┌──────────────┐      │
│  │ OrderPage  │──│  SeatGrid   │──│  Countdown   │──│ PaymentForm  │      │
│  └──────┬─────┘  └──────┬──────┘  └──────┬───────┘  └──────┬───────┘      │
│         │                │                 │                 │               │
│         │         (Select Seats)    (Timer Display)   (Submit Payment)      │
│         └────────────────┴─────────────────┴─────────────────┘              │
│                                   │                                          │
│                            useEventSource (SSE)                              │
│                                   │                                          │
└───────────────────────────────────┼──────────────────────────────────────────┘
                                    │
                          ┌─────────▼─────────┐
                          │   API Server      │
                          │   (Port 8080)     │
                          │                   │
                          │  HTTP Handlers:   │
                          │  • POST /orders   │
                          │  • POST /seats    │
                          │  • POST /payment  │
                          │  • GET  /events   │ ◄──── SSE Stream
                          │  • GET  /status   │
                          └─────────┬─────────┘
                                    │
                          ┌─────────▼──────────┐
                          │  Temporal Client   │
                          │  (Go SDK)          │
                          └─────────┬──────────┘
                                    │
        ┌───────────────────────────┼────────────────────────────┐
        │                           │                            │
        │              ┌────────────▼────────────┐              │
        │              │   TEMPORAL SERVER       │              │
        │              │   (Port 7233)           │              │
        │              │                         │              │
        │              │  • Workflow Engine      │              │
        │              │  • Event History        │              │
        │              │  • Task Queues          │              │
        │              │  • Timer Service        │              │
        │              └────────────┬────────────┘              │
        │                           │                            │
        │              ┌────────────▼────────────┐              │
        │              │   PostgreSQL Database   │              │
        │              │   (Temporal State)      │              │
        │              └─────────────────────────┘              │
        │                                                        │
        └────────────────────────────────────────────────────────┘
                                    │
                          ┌─────────▼──────────┐
                          │  Temporal Worker   │
                          │  (Go Process)      │
                          │                    │
                          │  Task Queues:      │
                          │  • order-tq        │
                          │  • seat-tq         │
                          └─────────┬──────────┘
                                    │
        ┌───────────────────────────┼────────────────────────────┐
        │                           │                            │
   ┌────▼─────┐          ┌─────────▼──────────┐      ┌────────▼─────────┐
   │ Order    │          │  Seat Entity       │      │  Payment         │
   │ Workflow │◄─signal──│  Workflows         │      │  Activities      │
   │          │          │  (One per seat)    │      │                  │
   │ • State  │          │                    │      │ • Validate       │
   │ • Timer  │──signal──│  • HOLD            │      │ • Confirm        │
   │ • Seats  │          │  • RELEASE         │      │ • Fail           │
   └──────────┘          └────────────────────┘      └──────────────────┘
```

---

## 🔄 State Flow Diagram

```
                         ┌───────────────┐
                         │   PENDING     │  ◄── Order Created
                         └───────┬───────┘
                                 │
                        (Select Seats Signal)
                                 │
                         ┌───────▼────────┐
                         │ SEATS_SELECTED │  ◄── 15-min Timer Starts
                         └───────┬────────┘
                                 │
                    ┌────────────┼────────────┐
                    │                         │
           (Submit Payment)              (Timer Expires)
                    │                         │
            ┌───────▼────────┐        ┌──────▼───────┐
            │  PROCESSING    │        │   EXPIRED    │
            │   PAYMENT      │        └──────────────┘
            └───────┬────────┘
                    │
         ┌──────────┼──────────┐
         │                     │
    (Success)             (3 Failures)
         │                     │
   ┌─────▼──────┐      ┌──────▼──────┐
   │ CONFIRMED  │      │   FAILED    │
   └────────────┘      └─────────────┘
```

---

## 🎯 Key Design Patterns

### 1. **Entity Workflow Pattern** (Seat Management)
Each seat is managed by its own long-running workflow:
- **Workflow ID**: `seat::{flightID}::{seatID}`
- **Purpose**: Serialize hold/release operations per seat
- **Benefits**: No database needed, Temporal handles concurrency

### 2. **Saga Pattern** (Order Orchestration)
The `OrderOrchestrationWorkflow` coordinates:
- Seat reservations (via signals to seat entities)
- Timer management (15-minute hold)
- Payment processing (with retries)
- State transitions

### 3. **Event Sourcing** (Real-time Updates)
- SSE endpoint streams workflow state changes
- UI subscribes to `/orders/{id}/events`
- No polling needed - real-time reactivity

### 4. **Activity Retry Pattern** (Payment)
```go
RetryPolicy: &temporal.RetryPolicy{
    MaximumAttempts:    3,
    InitialInterval:    1 * time.Second,
    MaximumInterval:    5 * time.Second,
    BackoffCoefficient: 2.0,
}
```

---

## 📂 Project Structure

```
temporal-seats/
├── cmd/
│   ├── api/          # API server entry point
│   └── worker/       # Temporal worker entry point
├── internal/
│   ├── activities/   # Payment, confirm, fail activities
│   ├── workflows/    # Order orchestration workflow
│   ├── entities/     # Seat entity workflow
│   ├── transport/    # HTTP handlers (REST + SSE)
│   └── domain/       # DTOs and domain types
├── test/
│   └── e2e/          # End-to-end integration tests
├── ui/
│   └── src/
│       ├── pages/     # OrderPage (main container)
│       ├── components/# SeatGrid, PaymentForm, Countdown
│       └── hooks/     # useEventSource (SSE hook)
├── infra/
│   └── docker-compose.yml  # Temporal + PostgreSQL
└── Makefile          # Build, test, run commands
```

---

## 🚀 Local Run

```bash
# 1. Start Docker containers (Temporal, etc.)
make up

# 2. Run Go services (API + Worker) in the background
make run

# 3. Open UI (optional - auto-starts on port 5173)
cd ui && npm install && npm run dev

# 4. (Optional) Stop the background Go services
make stop

# 5. (Optional) Stop and remove Docker containers
make down
```

### Access Points
- **UI**: http://localhost:5173
- **API**: http://localhost:8080
- **Temporal UI**: http://localhost:8088

---

## 🧪 Testing and Building

```bash
# Run all Go tests (unit + integration)
make test

# Run UI tests
make test-ui

# Run all tests (Go + UI)
make test-all

# Run end-to-end tests (requires Docker)
make test-e2e

# Compile the API and Worker into ./bin/
make build

# Run a full CI check (tidy, test, build)
make ci
```

### Test Coverage

```
Backend Go Tests:
├── Unit Tests: 15+ tests
│   ├── Order Workflow Tests (payment success/failure, retries, timer)
│   ├── Seat Entity Workflow Tests (hold, release, expiry)
│   ├── Payment Activity Tests (success, failure rate, timeout, concurrency)
│   └── HTTP Handler Tests (mocked Temporal client)
├── Integration Tests: API layer with real workflow execution
└── E2E Tests: Full system with Temporal, API, Worker, and SSE

UI Tests (Vitest + React Testing Library):
├── SeatGrid.test.tsx: Seat selection, debounce, confirmation states
├── PaymentForm.test.tsx: Input validation, submission, attempts
├── Countdown.test.tsx: Timer display, expiry states
├── OrderHeader.test.tsx: Status display, order info
└── OrderPage.test.tsx: Integration of all components
```

For detailed E2E test documentation, see [test/e2e/README.md](test/e2e/README.md).

---

## 📡 API Quick Demo

### 1. Create order

```bash
curl -s -XPOST localhost:8080/orders \
  -d '{"flightID":"F-100","orderID":"o-1"}' \
  -H 'Content-Type: application/json'
```

**Response:**
```json
{"orderID":"o-1"}
```

### 2. Select seats (starts/refreshes 15m hold)

```bash
curl -s -XPOST localhost:8080/orders/o-1/seats \
  -d '{"seats":["1A","1B"]}' \
  -H 'Content-Type: application/json'
```

**Response:**
```json
{"status":"seats updated"}
```

### 3. Watch status (SSE)

```bash
curl -N localhost:8080/orders/o-1/events
```

**Response (streaming):**
```
data: {"State":"PENDING","Seats":[],"HoldExpiresAt":"0001-01-01T00:00:00Z","AttemptsLeft":0}

data: {"State":"SEATS_SELECTED","Seats":["1A","1B"],"HoldExpiresAt":"2025-10-04T18:45:00Z","AttemptsLeft":3}
```

### 4. Pay

```bash
curl -s -XPOST localhost:8080/orders/o-1/payment \
  -d '{"code":"12345"}' \
  -H 'Content-Type: application/json'
```

**Response:**
```json
{"status":"payment submitted"}
```

### 5. Poll status (JSON)

```bash
curl -s localhost:8080/orders/o-1/status | jq
```

**Response:**
```json
{
  "State": "CONFIRMED",
  "Seats": ["1A", "1B"],
  "HoldExpiresAt": "2025-10-04T18:45:00Z",
  "AttemptsLeft": 3,
  "LastPaymentErr": ""
}
```

---

## ⚙️ Temporal Details

### Workflows

#### OrderOrchestrationWorkflow
- **Workflow ID**: `order::{orderID}`
- **Task Queue**: `order-tq`
- **Signals**:
  - `UpdateSeats`: Update selected seats and reset 15-minute timer
  - `SubmitPayment`: Process payment with retry logic
- **Query**:
  - `GetStatus`: Get current order state (used by SSE)
- **States**: `PENDING` → `SEATS_SELECTED` → `CONFIRMED`/`FAILED`/`EXPIRED`

#### SeatEntityWorkflow
- **Workflow ID**: `seat::{flightID}::{seatID}`
- **Task Queue**: `seat-tq`
- **Signals**:
  - `cmd` with `CommandType`:
    - `HOLD`: Reserve seat for an order (with TTL)
    - `EXTEND`: Extend existing hold
    - `RELEASE`: Release seat hold
- **Purpose**: Serialize seat operations, prevent double-booking

### Activities

#### ValidatePaymentActivity
- **Timeout**: 10 seconds
- **Retry Policy**: 3 attempts with exponential backoff
- **Behavior**: 15% random failure rate to simulate payment gateway flakiness
- **Input**: `orderID`, `paymentCode`
- **Output**: `"PAYMENT_SUCCESSFUL"` or error

#### ConfirmOrderActivity
- **Purpose**: Finalize order after successful payment
- **Side effects**: Could send confirmation email, update external systems

#### FailOrderActivity
- **Purpose**: Handle failed orders after exhausting payment retries
- **Side effects**: Could send failure notification, release holds

### Task Queues
- `order-tq`: Order orchestration workflows and payment activities
- `seat-tq`: Seat entity workflows

### Namespace
- `default` (configurable via environment)

---

## 🎨 UI Features

### Smart Seat Selection
- **Per-seat debounce** (300ms): Prevents accidental double-clicks
- **Rapid multi-selection**: Select multiple seats instantly
- **Visual feedback**:
  - Gray: Available
  - Cyan: Selected (local)
  - Orange: Confirming (API call in progress)
  - Green: Confirmed (backend acknowledged)

### Real-time Updates
- **SSE-powered**: No polling, instant updates
- **Countdown Timer**: Shows time remaining on 15-minute hold
- **State Transitions**: UI reflects backend state changes

### Payment Flow
- **5-digit code**: Numeric input validation
- **Attempt Tracking**: Shows remaining attempts (3 max)
- **Error Display**: Shows payment failure reasons
- **Auto-clear**: Input clears after submission

---

## 🛠️ Technical Highlights

### Backend (Go)
- **Temporal SDK**: Workflow orchestration and durable execution
- **Standard Library HTTP**: No heavy frameworks, simple REST API
- **Server-Sent Events**: Real-time state streaming
- **Structured Logging**: JSON logs with context

### Frontend (React + TypeScript)
- **Vite**: Fast build tool and dev server
- **Tailwind CSS**: Utility-first styling
- **Vitest**: Fast unit testing
- **Custom Hooks**: `useEventSource` for SSE

### Infrastructure
- **Docker Compose**: Local Temporal + PostgreSQL
- **Makefile**: Simplified dev workflow
- **GitHub Actions Ready**: CI/CD pipeline compatible

---

## 📝 Notes

### Design Decisions

1. **No Database**: Temporal Entity Workflows store seat state
2. **SSE over WebSocket**: Simpler, HTTP-native, auto-reconnect
3. **Per-seat Workflows**: Better concurrency, no locking needed
4. **Saga Pattern**: Order workflow coordinates all operations
5. **Retry Policies**: Built-in Temporal retries for payment flakiness

### Production Considerations

For a production deployment, consider:
- **Database**: Add persistent storage for audit logs
- **Authentication**: Add JWT/OAuth for API security
- **Rate Limiting**: Protect API endpoints
- **Monitoring**: Add metrics (Prometheus) and tracing (Jaeger)
- **Horizontal Scaling**: Multiple workers for high throughput
- **Circuit Breakers**: Protect against cascading failures

---

## 📚 Further Reading

- [Temporal Documentation](https://docs.temporal.io/)
- [Entity Workflow Pattern](https://docs.temporal.io/encyclopedia/entity-workflows)
- [Saga Pattern](https://docs.temporal.io/encyclopedia/saga-pattern)
- [Server-Sent Events](https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events)

---

## 📄 License

This is a home assignment project for demonstration purposes.
