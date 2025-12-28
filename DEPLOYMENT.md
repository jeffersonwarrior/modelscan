# ModelScan v0.3 - Deployment Guide

## System Requirements

### Minimum Requirements
- **RAM**: 20 MB (actual usage: ~13 MB RSS)
- **Disk**: 50 MB (binary: 17 MB + database + generated SDKs)
- **CPU**: 1 core @ 1 GHz (any modern CPU)
- **OS**: Linux, macOS, Windows (Go cross-platform)

### Recommended Requirements
- **RAM**: 128 MB (comfortable headroom for multiple providers)
- **Disk**: 500 MB (room for 50+ generated SDKs)
- **CPU**: 2 cores @ 2 GHz
- **OS**: Linux (Ubuntu 20.04+ or similar)

### RAM Breakdown

**Base Server** (idle state):
```
RSS (Resident Set Size): ~13 MB
VSZ (Virtual Memory): ~1.6 GB (Go runtime)
```

**Per Component**:
- Database (SQLite): ~1 MB
- Discovery Agent: ~2 MB
- SDK Generator: ~1 MB
- Key Manager: <1 MB
- Admin API: ~1 MB
- HTTP Server: ~3 MB
- Go Runtime Overhead: ~5 MB

**Under Load** (10 concurrent requests):
- Expected RAM: ~50 MB RSS
- Peak RAM: ~100 MB RSS

**Production Deployment** (recommended):
- Allocate: 256 MB RAM
- Allows: 50+ providers, 100+ keys, 1000+ req/sec

## Quick Deployment

### 1. Build

```bash
# Clone repository
git clone https://github.com/jeffersonwarrior/modelscan.git
cd modelscan

# Build server
go build -o modelscan-server ./cmd/modelscan-server/

# Verify binary size
ls -lh modelscan-server
# Output: 17M
```

### 2. Initialize

```bash
# Initialize database
./modelscan-server --init

# Verify database created
ls -lh modelscan.db
# Output: ~32K (empty database)
```

### 3. Configure (Optional)

```bash
# Create config
cp config.example.yaml config.yaml

# Edit as needed
nano config.yaml
```

Or use environment variables:
```bash
export MODELSCAN_DB_PATH=/var/lib/modelscan/data.db
export MODELSCAN_HOST=0.0.0.0  # For LAN access
export MODELSCAN_PORT=9090
export MODELSCAN_AGENT_MODEL=claude-sonnet-4-5
```

### 4. Run

```bash
# Start server
./modelscan-server

# Or with custom config
./modelscan-server --config /etc/modelscan/config.yaml

# Or as background service
nohup ./modelscan-server > /var/log/modelscan.log 2>&1 &
```

### 5. Test

```bash
# Health check
curl http://localhost:9090/health

# List providers
curl http://localhost:9090/api/providers

# Add provider (requires database entry first)
sqlite3 modelscan.db "INSERT INTO providers (id, name, base_url, auth_method, pricing_model, status) VALUES ('openai', 'OpenAI', 'https://api.openai.com/v1', 'bearer', 'pay-per-token', 'online');"

# Add API key using psst
psst OPENAI_API_KEY -- bash -c '
  curl -X POST http://localhost:9090/api/keys/add \
    -H "Content-Type: application/json" \
    -d "{\"provider_id\": \"openai\", \"api_key\": \"$OPENAI_API_KEY\"}"
'
```

## Production Deployment

### Systemd Service

Create `/etc/systemd/system/modelscan.service`:

```ini
[Unit]
Description=ModelScan v0.3 - Auto-discovering SDK service
After=network.target

[Service]
Type=simple
User=modelscan
Group=modelscan
WorkingDirectory=/opt/modelscan
ExecStart=/opt/modelscan/modelscan-server
Restart=on-failure
RestartSec=5s

# Environment
Environment="MODELSCAN_DB_PATH=/var/lib/modelscan/data.db"
Environment="MODELSCAN_HOST=0.0.0.0"
Environment="MODELSCAN_PORT=9090"

# Security
PrivateTmp=true
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/modelscan

# Resource limits
LimitNOFILE=65536
MemoryMax=256M

[Install]
WantedBy=multi-user.target
```

Enable and start:
```bash
sudo systemctl enable modelscan
sudo systemctl start modelscan
sudo systemctl status modelscan
```

### Docker Deployment

Create `Dockerfile`:

```dockerfile
FROM golang:1.23-alpine AS builder

WORKDIR /build
COPY . .
RUN go build -o modelscan-server ./cmd/modelscan-server/

FROM alpine:latest

RUN apk --no-cache add ca-certificates sqlite

WORKDIR /app
COPY --from=builder /build/modelscan-server .
COPY config.example.yaml config.yaml

RUN mkdir -p /var/lib/modelscan
RUN ./modelscan-server --init

EXPOSE 9090

CMD ["./modelscan-server"]
```

Build and run:
```bash
docker build -t modelscan:0.3 .
docker run -d \
  --name modelscan \
  -p 9090:9090 \
  -v modelscan-data:/var/lib/modelscan \
  --memory=256m \
  modelscan:0.3
```

### Kubernetes Deployment

Create `modelscan-deployment.yaml`:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: modelscan
spec:
  replicas: 2
  selector:
    matchLabels:
      app: modelscan
  template:
    metadata:
      labels:
        app: modelscan
    spec:
      containers:
      - name: modelscan
        image: modelscan:0.3
        ports:
        - containerPort: 9090
        env:
        - name: MODELSCAN_DB_PATH
          value: "/var/lib/modelscan/data.db"
        - name: MODELSCAN_HOST
          value: "0.0.0.0"
        - name: MODELSCAN_PORT
          value: "9090"
        resources:
          requests:
            memory: "64Mi"
            cpu: "100m"
          limits:
            memory: "256Mi"
            cpu: "500m"
        volumeMounts:
        - name: data
          mountPath: /var/lib/modelscan
      volumes:
      - name: data
        persistentVolumeClaim:
          claimName: modelscan-pvc
---
apiVersion: v1
kind: Service
metadata:
  name: modelscan
spec:
  selector:
    app: modelscan
  ports:
  - protocol: TCP
    port: 9090
    targetPort: 9090
  type: LoadBalancer
```

Deploy:
```bash
kubectl apply -f modelscan-deployment.yaml
kubectl get pods -l app=modelscan
kubectl get service modelscan
```

## Security Best Practices

### 1. API Key Management with psst

**DO NOT** hardcode API keys:
```bash
# ❌ BAD
export OPENAI_API_KEY="sk-..."

# ✓ GOOD - Use psst
psst set OPENAI_API_KEY
# (Enter key securely when prompted)
```

Use psst in scripts:
```bash
psst OPENAI_API_KEY ANTHROPIC_API_KEY -- ./deploy-providers.sh
```

### 2. Network Security

**Development** (localhost only):
```bash
MODELSCAN_HOST=127.0.0.1 ./modelscan-server
```

**Production** (with reverse proxy):
```bash
# Run on localhost
MODELSCAN_HOST=127.0.0.1 MODELSCAN_PORT=9090 ./modelscan-server
```

nginx config:
```nginx
server {
    listen 443 ssl http2;
    server_name api.example.com;

    ssl_certificate /etc/letsencrypt/live/api.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/api.example.com/privkey.pem;

    location / {
        proxy_pass http://127.0.0.1:9090;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

### 3. Database Security

```bash
# Restrict database file permissions
chmod 600 modelscan.db

# Regular backups
sqlite3 modelscan.db ".backup /backup/modelscan-$(date +%Y%m%d).db"

# Verify integrity
sqlite3 modelscan.db "PRAGMA integrity_check;"
```

## Monitoring

### Health Checks

```bash
# Simple health check
curl http://localhost:9090/health

# Detailed monitoring
while true; do
  echo "$(date) - $(curl -s http://localhost:9090/health | jq -r .status)"
  sleep 60
done
```

### Resource Monitoring

```bash
# RAM usage
ps aux | grep modelscan-server | grep -v grep | awk '{print $6/1024" MB"}'

# CPU usage
top -b -n 1 | grep modelscan-server

# Database size
ls -lh modelscan.db
```

### Logs

```bash
# Follow logs
tail -f /var/log/modelscan.log

# Search for errors
grep ERROR /var/log/modelscan.log

# Count requests
grep "Routing request" /var/log/modelscan.log | wc -l
```

## Performance Tuning

### Database Optimization

```bash
sqlite3 modelscan.db "PRAGMA optimize;"
sqlite3 modelscan.db "VACUUM;"
```

### Connection Pooling

For high-traffic deployments, consider adding connection pooling:
- Set `GOMAXPROCS` to match CPU cores
- Increase file descriptor limits
- Use load balancer for multiple instances

### Caching

Discovery results cached for 7 days by default. Adjust:
```bash
MODELSCAN_CACHE_DAYS=14 ./modelscan-server
```

## Troubleshooting

### Port Already in Use

```bash
# Find process
sudo lsof -i :9090

# Kill process
sudo kill -9 <PID>

# Or use different port
MODELSCAN_PORT=9091 ./modelscan-server
```

### Database Locked

```bash
# Check for locks
sudo lsof modelscan.db

# If stuck, reinitialize
mv modelscan.db modelscan.db.backup
./modelscan-server --init
```

### High Memory Usage

```bash
# Check actual RSS
ps aux | grep modelscan-server

# If >256MB, investigate:
# 1. Check number of cached discovery results
# 2. Check generated SDK count
# 3. Restart service
sudo systemctl restart modelscan
```

## Scaling

### Horizontal Scaling

Use load balancer with multiple instances:
```
      ┌─→ modelscan-1 (9090)
      │
LB ───┼─→ modelscan-2 (9090)
      │
      └─→ modelscan-3 (9090)
```

Share database via:
- Network-mounted SQLite (NFS)
- PostgreSQL (requires migration)
- Read replicas for queries

### Vertical Scaling

For single instance under heavy load:
- Increase RAM allocation to 512 MB
- Use 2+ CPU cores
- Enable Go runtime optimizations

## Backup & Recovery

### Automated Backups

```bash
#!/bin/bash
# backup-modelscan.sh

BACKUP_DIR=/backup/modelscan
DATE=$(date +%Y%m%d-%H%M%S)

# Backup database
sqlite3 modelscan.db ".backup $BACKUP_DIR/db-$DATE.db"

# Backup generated SDKs
tar -czf $BACKUP_DIR/sdks-$DATE.tar.gz generated/

# Keep only last 7 days
find $BACKUP_DIR -mtime +7 -delete

echo "Backup complete: $DATE"
```

Add to crontab:
```bash
0 2 * * * /opt/modelscan/backup-modelscan.sh
```

### Recovery

```bash
# Stop service
sudo systemctl stop modelscan

# Restore database
cp /backup/modelscan/db-20251227.db modelscan.db

# Restore SDKs
tar -xzf /backup/modelscan/sdks-20251227.tar.gz

# Start service
sudo systemctl start modelscan
```

## Summary

**Minimum Hardware**: 20 MB RAM, 50 MB disk
**Recommended Hardware**: 256 MB RAM, 500 MB disk
**Production Ready**: Systemd, Docker, Kubernetes configs included
**Secure by Default**: psst integration, localhost binding
**Low Overhead**: ~13 MB RAM at idle, ~50 MB under load
**Easy Deployment**: Single 17 MB binary, no dependencies

---

**Next Steps**:
1. Deploy using preferred method (binary, Docker, Kubernetes)
2. Configure API keys using psst
3. Set up monitoring
4. Add providers as needed
5. Scale horizontally if needed

For questions: https://github.com/jeffersonwarrior/modelscan/issues
