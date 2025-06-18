# Stage 1: Build the Go application
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Copy the Go source file into the builder stage
COPY main.go .

RUN echo "Pausing Docker BUILD for 15 minutes..." && sleep 600 && echo "Resuming Docker BUILD."

# Build the Go application
# -o cgroup_info: Specifies the output executable name
# -ldflags "-s -w": Reduces the size of the executable by omitting debug information
RUN go build -o cgroup_info -ldflags "-s -w" main.go

# Stage 2: Create the final, minimal image
FROM registry.access.redhat.com/ubi9/ubi:9.6-1749542372

WORKDIR /app

# Copy the compiled Go executable from the builder stage
COPY --from=builder /app/cgroup_info .

# Set the entrypoint for the container to run the executable
CMD ["./cgroup_info"]
