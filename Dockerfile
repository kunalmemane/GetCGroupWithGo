# Stage 1: Build the Go application
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Copy the Go source file into the builder stage
COPY main.go .

# Build the Go application
# -o cgroup_info: Specifies the output executable name
# -ldflags "-s -w": Reduces the size of the executable by omitting debug information
RUN go build -o cgroup_info -ldflags "-s -w" main.go

# Stage 2: Create the final, minimal image
FROM alpine:latest

WORKDIR /app

# Copy the compiled Go executable from the builder stage
COPY --from=builder /app/cgroup_info .


# Define an ENTRYPOINT that will first print cgroup information, including cpu.max,
# and then execute the main application binary.
ENTRYPOINT [ "/bin/sh", "-c", "echo '---------------------------------------------'; \
                                echo '      Cgroup Info from within Container      '; \
                                echo '---------------------------------------------'; \
                                echo 'Current Process Cgroups:'; \
                                cat /proc/self/cgroup; \
                                echo ''; \
                                echo 'Cgroup Filesystem Mounts:'; \
                                mount | grep cgroup || true; \
                                echo ''; \
                                echo 'CPU Usage Limits (cgroup v2 - cpu.max):'; \
                                CGROUP_V2_PATH=$(cat /proc/self/cgroup | grep '^0::' | awk -F':' '{print $3}'); \
                                if [ -n \"$CGROUP_V2_PATH\" ]; then \
                                    CPU_MAX_FILE=\"/sys/fs/cgroup${CGROUP_V2_PATH}/cpu.max\"; \
                                    echo \"Attempting to read: $CPU_MAX_FILE\"; \
                                    if [ -f \"$CPU_MAX_FILE\" ]; then \
                                        echo \"cpu.max: $(cat $CPU_MAX_FILE)\"; \
                                    else \
                                        echo \"Warning: $CPU_MAX_FILE not found or accessible. This might mean cgroup v2 is not active, or the path is different, or no CPU limit is set.\"; \
                                    fi; \
                                else \
                                    echo \"Warning: Could not determine cgroup v2 path from /proc/self/cgroup.\"; \
                                    echo \"(cgroup v1 CPU limits typically use cpu.cfs_quota_us and cpu.cfs_period_us under /sys/fs/cgroup/cpu)\"; \
                                fi; \
                                echo ''; \
                                echo '---------------------------------------------'; \
                                echo '        Starting Go Application              '; \
                                echo '---------------------------------------------'; \
                                exec \"$@\"" ]


# Set the entrypoint for the container to run the executable
CMD ["./cgroup_info"]
