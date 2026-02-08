# Resource Optimization via Overcommit

## Summary

After testing, we discovered that aggressive CPU/memory overcommit achieves similar density gains as a multi-user architecture shift—without any code changes.

## Key Findings

### Actual Resource Usage (4 agents)

| State | CPU per agent | Memory per agent |
|-------|---------------|------------------|
| Idle | 1m | 36-40Mi |
| Active | 50-120m | 170-300Mi |
| Peak (burst) | 150-200m | 300-400Mi |

### Current Defaults (after initial optimization)

```
CPU:    request=250m,  limit=1000m
Memory: request=512Mi, limit=1Gi
```

On 4-core / 8GB cluster: **~15-16 agents max**

### Opportunity

Since actual idle usage is ~1m CPU and ~40Mi memory, we can aggressively overcommit:

| Config | CPU Req | Mem Req | Max Agents (8GB) | Risk Level |
|--------|---------|---------|------------------|------------|
| Conservative | 250m | 512Mi | 16 | Very Low |
| Moderate | 100m | 256Mi | 32 | Low |
| Aggressive | 50m | 128Mi | 62 | Medium |
| Extreme | 25m | 64Mi | 125 | High |

---

## Stress Testing Plan

### Goal

Find the optimal request/limit configuration that maximizes agent density while maintaining acceptable performance under load.

### Test Matrix

| Test | CPU Request | Memory Request | CPU Limit | Memory Limit |
|------|-------------|----------------|-----------|--------------|
| T1 (baseline) | 250m | 512Mi | 1000m | 1Gi |
| T2 | 100m | 256Mi | 1000m | 1Gi |
| T3 | 50m | 128Mi | 1000m | 1Gi |
| T4 | 25m | 64Mi | 1000m | 1Gi |
| T5 | 100m | 128Mi | 2000m | 2Gi |

### Test Scenarios

#### Scenario A: Idle Density
- Create maximum number of agents
- All agents idle (no active sessions)
- Measure: How many can be scheduled? Any OOM kills?

```bash
# Create agents until scheduling fails
for i in $(seq 1 50); do
  vibespace test-stress agent create -t claude-code --skip-permissions || break
  echo "Created agent $i"
done
```

#### Scenario B: Single Active
- Create N agents (e.g., 20)
- Activate 1 agent with intensive task
- Measure: Does active agent get enough resources? Latency?

```bash
# Create 20 agents
vibespace create stress-test -t claude-code --skip-permissions --cpu 100m --memory 128Mi

for i in $(seq 2 20); do
  vibespace stress-test agent create -t claude-code --skip-permissions
done

# Connect to one and run intensive task
vibespace stress-test connect claude-1
# Run: "Read all files in this directory and summarize each one"
```

#### Scenario C: Multiple Active
- Create N agents (e.g., 20)
- Activate 5 agents simultaneously
- Measure: Performance degradation? OOM kills?

```bash
# Open 5 browser sessions to different agents
vibespace stress-test connect claude-1 --browser &
vibespace stress-test connect claude-2 --browser &
vibespace stress-test connect claude-3 --browser &
vibespace stress-test connect claude-4 --browser &
vibespace stress-test connect claude-5 --browser &

# Send same prompt to all: "Explain the theory of relativity in detail"
```

#### Scenario D: Burst Storm
- Create N agents
- Activate ALL agents at once
- Measure: How many survive? Recovery time?

### Metrics to Collect

```bash
# Continuous monitoring during tests
watch -n 2 'kubectl top pods -n vibespace && echo "" && kubectl top nodes'

# Check for OOM kills
kubectl get events -n vibespace --field-selector reason=OOMKilled

# Check for evictions
kubectl get events -n vibespace --field-selector reason=Evicted

# Pod restart counts
kubectl get pods -n vibespace -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.status.containerStatuses[0].restartCount}{"\n"}{end}'
```

### Recording Results

| Test | Agents Created | Agents Scheduled | OOM Kills | Evictions | Active Latency | Notes |
|------|----------------|------------------|-----------|-----------|----------------|-------|
| T1-A | | | | | | |
| T1-B | | | | | | |
| T1-C | | | | | | |
| T2-A | | | | | | |
| ... | | | | | | |

---

## Risk Mitigation

### OOM Risk

When memory is overcommitted, simultaneous activation can cause OOM:

```
20 agents × 300Mi active = 6GB (OK on 8GB)
40 agents × 300Mi active = 12GB (OOM on 8GB)
```

**Mitigations:**
1. Set memory limit equal to worst-case active usage (1Gi)
2. Use Pod Priority classes - evict low-priority agents first
3. Monitor and alert on memory pressure
4. Document recommended max agents per cluster size

### CPU Throttling

When CPU is overcommitted, active agents may be throttled:

```
20 agents × 100m active = 2000m (OK on 4-core)
20 agents × 500m active = 10000m (throttled on 4-core)
```

**Mitigations:**
1. CPU throttling is graceful (unlike OOM kill)
2. Higher limits allow burst when others are idle
3. Document expected latency at different densities

---

## Recommended Configurations

### Small Cluster (4 CPU / 8GB)

```yaml
# Conservative (up to 16 agents)
resources:
  requests:
    cpu: 250m
    memory: 512Mi
  limits:
    cpu: 1000m
    memory: 1Gi

# Moderate (up to 30 agents) - RECOMMENDED
resources:
  requests:
    cpu: 100m
    memory: 256Mi
  limits:
    cpu: 1000m
    memory: 1Gi
```

### Medium Cluster (8 CPU / 16GB)

```yaml
# Moderate (up to 60 agents)
resources:
  requests:
    cpu: 100m
    memory: 256Mi
  limits:
    cpu: 1000m
    memory: 1Gi

# Aggressive (up to 100 agents)
resources:
  requests:
    cpu: 50m
    memory: 128Mi
  limits:
    cpu: 1000m
    memory: 1Gi
```

### Large Cluster (16 CPU / 32GB)

```yaml
# Aggressive (up to 200 agents)
resources:
  requests:
    cpu: 50m
    memory: 128Mi
  limits:
    cpu: 2000m
    memory: 2Gi
```

---

## Implementation

### Phase 1: Update Defaults

After stress testing confirms optimal values:

```go
// internal/cli/create.go
const (
    DefaultCPU         = "100m"   // was 250m
    DefaultCPULimit    = "1000m"
    DefaultMemory      = "256Mi"  // was 512Mi
    DefaultMemoryLimit = "1Gi"
)
```

### Phase 2: Cluster Size Presets

Add `--preset` flag for common configurations:

```bash
vibespace create myproject -t claude-code --preset small   # Conservative
vibespace create myproject -t claude-code --preset medium  # Moderate
vibespace create myproject -t claude-code --preset dense   # Aggressive
```

### Phase 3: Documentation

- Document recommended agent counts per cluster size
- Add warnings when approaching density limits
- Add `vibespace doctor` check for overcommit risk

---

## Why Not Multi-User Architecture?

We considered a multi-user model (multiple Unix users in one container) but determined overcommit achieves similar density with less complexity:

| Approach | Max Agents | Complexity | Isolation |
|----------|------------|------------|-----------|
| No overcommit | 4 | Low | High |
| Moderate overcommit | 30 | Low | High |
| Aggressive overcommit | 60+ | Low | High |
| Multi-user | 60+ | High | Low |

**Conclusion:** Stick with current architecture, tune resource requests.

---

## Next Steps

1. [ ] Run stress test matrix (T1-T5 × Scenarios A-D)
2. [ ] Record results in table above
3. [ ] Determine recommended defaults
4. [ ] Update `DefaultCPU` and `DefaultMemory` constants
5. [ ] Add `--preset` flag for cluster size configurations
6. [ ] Document recommended agent counts in README
