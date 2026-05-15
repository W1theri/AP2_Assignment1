# Assignment 4 - Architecture Diagram (Mermaid)

## Cache-Aside Pattern Flow

```mermaid
sequenceDiagram
    participant Client
    participant OrderService as Order Service<br/>(Rate Limiter)
    participant Cache as Redis Cache<br/>(TTL: 5 min)
    participant DB as PostgreSQL<br/>(Orders)
    participant Payment as Payment<br/>Service

    Client->>OrderService: GET /orders/:id
    
    rect rgb(200, 220, 255)
    Note over OrderService,Cache: Rate Limit Check
    OrderService->>Cache: INCR rate_limit:IP
    Cache-->>OrderService: Count
    end
    
    rect rgb(255, 220, 200)
    Note over OrderService,Cache: Cache-Aside Pattern
    OrderService->>Cache: GET order:id
    alt Cache Hit
        Cache-->>OrderService: Order JSON
        OrderService-->>Client: 200 OK (from cache)
    else Cache Miss
        Cache-->>OrderService: nil
        OrderService->>DB: SELECT * FROM orders WHERE id=?
        DB-->>OrderService: Order row
        OrderService->>Cache: SET order:id JSON (300s)
        Cache-->>OrderService: OK
        OrderService-->>Client: 200 OK (fresh)
    end
    end

    Client->>OrderService: PATCH /orders/:id/pay
    OrderService->>Payment: gRPC ProcessPayment
    Payment-->>OrderService: Authorized
    OrderService->>DB: UPDATE orders SET status='Paid'
    
    rect rgb(200, 255, 200)
    Note over OrderService,Cache: Cache Invalidation
    OrderService->>Cache: DEL order:id
    OrderService->>Cache: DEL orders:recent:10
    Cache-->>OrderService: OK
    OrderService-->>Client: 200 OK
    Note over OrderService: [CACHE INVALIDATED]<br/>Next GET will fetch from DB
    end
```

## Background Job Processing with Retry Logic

```mermaid
sequenceDiagram
    participant RabbitMQ
    participant Consumer as Notification Service<br/>(Enhanced Consumer)
    participant Redis as Redis<br/>(Idempotency)
    participant Retry as Retry Logic<br/>(Exp. Backoff)
    participant Provider as Email Provider<br/>(Simulated/Real)

    RabbitMQ->>Consumer: Deliver payment.completed<br/>(PaymentEvent)
    
    rect rgb(255, 240, 200)
    Note over Consumer,Redis: Idempotency Check
    Consumer->>Redis: EXISTS idempotency:event_id
    alt Already Processed
        Redis-->>Consumer: 1 (exists)
        Consumer->>RabbitMQ: ACK (skip)
        Note over Consumer: [DUPLICATE] Skipped
    else First Time
        Redis-->>Consumer: 0 (not exists)
        Consumer->>Consumer: Parse event
    end
    end
    
    rect rgb(200, 220, 255)
    Note over Consumer,Provider: Retry Loop (Max 5 attempts)
    
    loop Max 5 Attempts
        Consumer->>Retry: ExecuteWithRetry(sendEmail)
        activate Retry
        
        alt Attempt Success
            Retry->>Provider: Send(to, subject, body)
            Note over Provider: [SIMULATED EMAIL]<br/>Latency: 500ms ±30%<br/>Failure rate: 10%
            Provider-->>Retry: nil (success)
            Retry-->>Consumer: success
            deactivate Retry
            break
        else Attempt Fails
            Retry->>Provider: Send(to, subject, body)
            Provider-->>Retry: error (simulated)
            Retry->>Retry: Calculate backoff<br/>2s → 4s → 8s → 16s → 32s
            Retry-->>Consumer: wait N seconds
            Note over Retry: Attempt {i} failed, retrying in {delay}
        end
    end
    end
    
    rect rgb(200, 255, 200)
    Note over Consumer,Redis: Mark Processed & ACK
    Consumer->>Redis: SET idempotency:event_id processed (24h TTL)
    Redis-->>Consumer: OK
    Consumer->>RabbitMQ: ACK
    Note over RabbitMQ: Message removed from queue
    end
```

## Rate Limiter Flow

```mermaid
sequenceDiagram
    participant ClientA as Client A<br/>(IP: 192.168.1.10)
    participant ClientB as Client B<br/>(IP: 192.168.1.20)
    participant RateLimiter as Rate Limiter<br/>Middleware
    participant Redis as Redis<br/>Counter Store
    participant Handler as HTTP Handler

    rect rgb(255, 200, 200)
    Note over ClientA,Handler: Rate Limit: 10 req/min per IP
    end

    loop Request Sequence
        ClientA->>RateLimiter: GET /orders/recent
        RateLimiter->>RateLimiter: Extract IP: 192.168.1.10
        RateLimiter->>Redis: INCR rate_limit:192.168.1.10
        Redis-->>RateLimiter: Count (1-10)
        
        alt Count ≤ Limit
            RateLimiter->>Handler: next()
            Handler-->>ClientA: 200 OK
        else Count > Limit
            RateLimiter-->>ClientA: 429 Too Many Requests<br/>{"retry_after": 60}
        end
        
        ClientB->>RateLimiter: GET /orders/{id}
        RateLimiter->>RateLimiter: Extract IP: 192.168.1.20
        RateLimiter->>Redis: INCR rate_limit:192.168.1.20
        Redis-->>RateLimiter: Count (independent for B)
        RateLimiter->>Handler: next()
        Handler-->>ClientB: 200 OK
    end
```

## Exponential Backoff with Jitter

```mermaid
graph TD
    A["Attempt 1<br/>Send Email"] -->|Fail| B["Wait 2s ±10%<br/>(1.8s-2.2s)"]
    B --> C["Attempt 2<br/>Send Email"]
    C -->|Fail| D["Wait 4s ±10%<br/>(3.6s-4.4s)"]
    D --> E["Attempt 3<br/>Send Email"]
    E -->|Fail| F["Wait 8s ±10%<br/>(7.2s-8.8s)"]
    F --> G["Attempt 4<br/>Send Email"]
    G -->|Fail| H["Wait 16s ±10%<br/>(14.4s-17.6s)"]
    H --> I["Attempt 5<br/>Send Email"]
    I -->|Fail| J["Wait 32s ±10%<br/>(28.8s-35.2s)"]
    J --> K["Max Retries Reached<br/>NACK with Requeue"]
    
    A -->|Success| L["Mark Processed<br/>ACK Message"]
    C -->|Success| L
    E -->|Success| L
    G -->|Success| L
    I -->|Success| L
    
    style A fill:#90EE90
    style C fill:#90EE90
    style E fill:#90EE90
    style G fill:#90EE90
    style I fill:#90EE90
    style L fill:#FFD700
    style K fill:#FF6B6B
```

## Deployment Topology

```mermaid
graph TB
    subgraph Client["Client Layer"]
        REST["REST Client<br/>Port 8080"]
    end
    
    subgraph OrderSvc["Order Service<br/>Port 8080, 50052"]
        RL["Rate Limiter<br/>Middleware"]
        Handler["HTTP Handlers"]
        UC["UseCase Layer"]
        CachedRepo["CachedOrderRepository<br/>Cache-Aside Pattern"]
    end
    
    subgraph Caching["Caching Layer"]
        Redis[("Redis:6379<br/>- Order cache<br/>- Rate limit counters<br/>- Idempotency store")]
    end
    
    subgraph Persistence["Persistence Layer"]
        ODB[("PostgreSQL:5432<br/>order_db")]
        PDB[("PostgreSQL:5433<br/>payment_db")]
    end
    
    subgraph PaymentSvc["Payment Service<br/>Port 8081, 50051"]
        PHandler["gRPC Handler"]
        PUC["UseCase Layer"]
        PRepo["Repository"]
    end
    
    subgraph Messaging["Message Broker"]
        RMQ["RabbitMQ:5672<br/>Queue: payment.completed"]
    end
    
    subgraph NotifSvc["Notification Service"]
        Consumer["Enhanced Consumer<br/>- Retry Logic<br/>- Idempotency Check"]
        Retry["Exponential Backoff<br/>Max 5 retries"]
        Provider["Provider Adapter<br/>Simulated/Real Email"]
    end
    
    REST --> RL
    RL --> Handler
    Handler --> UC
    UC --> CachedRepo
    CachedRepo --> Redis
    CachedRepo --> ODB
    
    UC -.->|gRPC| PHandler
    PHandler --> PUC
    PUC --> PRepo
    PRepo --> PDB
    
    PUC --> RMQ
    
    RMQ --> Consumer
    Consumer --> Redis
    Consumer --> Retry
    Retry --> Provider
    
    style Redis fill:#FF6B6B,color:#fff
    style RMQ fill:#FF8C42,color:#fff
    style Caching fill:#E8F4F8
    style Persistence fill:#F0E8F4
    style OrderSvc fill:#E8F8E8
    style PaymentSvc fill:#F8E8E8
    style NotifSvc fill:#F8F8E8
```

## Invalidation Strategy

```mermaid
stateDiagram-v2
    [*] --> Pending: Create Order
    
    Pending --> CheckCache: GET /orders/:id
    CheckCache --> ReturnCached: Cache Hit<br/>(TTL: 5 min)
    CheckCache --> FetchDB: Cache Miss
    FetchDB --> StoreCache: Store in Redis
    StoreCache --> ReturnFresh: Return to Client
    
    Pending --> ProcessPayment: Process Payment<br/>(gRPC)
    ProcessPayment --> UpdateDB: Update status:<br/>Pending → Paid
    UpdateDB --> InvalidateCache: DELETE order:id
    InvalidateCache --> InvalidateRecent: DELETE orders:recent:*
    InvalidateRecent --> Paid: Order Paid
    
    Paid --> CheckCache2: GET /orders/:id
    CheckCache2 --> FreshFetch: Cache Invalidated<br/>Must query DB
    FreshFetch --> StoreCache
    
    ReturnFresh --> [*]
    ReturnCached --> [*]
    
    style Pending fill:#FFEB3B
    style Paid fill:#4CAF50,color:#fff
    style InvalidateCache fill:#FF6B6B,color:#fff
    style InvalidateRecent fill:#FF6B6B,color:#fff
```

