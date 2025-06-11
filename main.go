package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	// Common cgroup mount point for v2 
	cgroupV2Mountpoint = "/sys/fs/cgroup"  
	// Typical cgroup mount point for v1 CPU controller 
	cgroupV1Mountpoint = "/sys/fs/cgroup/cpu,cpuacct" 

	// cgroup v2 filenames
	cpuMaxV2Filename    = "cpu.max"
	cpuWeightV2Filename = "cpu.weight" // While not strictly asked, good to know

	// cgroup v1 filenames
	cpuQuotaV1Filename  = "cpu.cfs_quota_us"
	cpuPeriodV1Filename = "cpu.cfs_period_us"
	cpuSharesV1Filename = "cpu.shares"

	port = ":8080" // Port the web server will listen on
)

// CgroupInfo holds the parsed CPU limit information
type CgroupInfo struct {
	CgroupVersion          string
	CPUMax                 string // For v2: cpu.max, For v1: calculated quota in microseconds
	CPUPeriod              string // For v2: from cpu.max, For v1: cpu.cfs_period_us
	CPUShares              string // For v1: cpu.shares, For v2: n/a
	CPUWeight              string // For v2: cpu.weight, For v1: n/a
	BurstableCPUPercentage string
	Error                  string
}

// detectCgroupVersion checks which cgroup version is active for the current process
func detectCgroupVersion() string {
	// Check for cgroup v2 unified hierarchy
	// If /sys/fs/cgroup/cgroup.controllers exists and is readable, it's likely v2
	if _, err := os.Stat(filepath.Join(cgroupV2Mountpoint, "cgroup.controllers")); err == nil {
		return "v2"
	}
	// Check for cgroup v1 cpuacct controller
	if _, err := os.Stat(filepath.Join(cgroupV1Mountpoint, cpuQuotaV1Filename)); err == nil {
		return "v1"
	}
	return "unknown"
}

// getCPUMaxInfoV2 reads CPU limits from cgroup v2 files
func getCPUMaxInfoV2() CgroupInfo {
	info := CgroupInfo{CgroupVersion: "v2"}
	cpuMaxPath := filepath.Join(cgroupV2Mountpoint, cpuMaxV2Filename)

	content, err := ioutil.ReadFile(cpuMaxPath)
	if err != nil {
		info.Error = fmt.Sprintf("Error reading %s: %v", cpuMaxPath, err)
		return info
	}

	parts := strings.Fields(string(content))
	if len(parts) < 2 {
		info.Error = fmt.Sprintf("Error: Unexpected format in %s: %s", cpuMaxPath, string(content))
		return info
	}

	maxValueStr := parts[0]
	periodStr := parts[1]

	info.CPUPeriod = fmt.Sprintf("%s microseconds", periodStr)

	var maxValue int64
	if maxValueStr == "max" {
		info.CPUMax = "unlimited (max)"
		info.BurstableCPUPercentage = "N/A (unlimited)"
	} else {
		maxValue, err = strconv.ParseInt(maxValueStr, 10, 64)
		if err != nil {
			info.Error = fmt.Sprintf("Error parsing max value from %s: %v", cpuMaxPath, err)
			return info
		}
		info.CPUMax = fmt.Sprintf("%d microseconds", maxValue)

		period, err := strconv.ParseInt(periodStr, 10, 64)
		if err != nil {
			info.Error = fmt.Sprintf("Error parsing period value from %s: %v", cpuMaxPath, err)
			return info
		}

		if period > 0 {
			burstablePercentage := float64(maxValue) / float64(period) * 100
			info.BurstableCPUPercentage = fmt.Sprintf("%.2f%%", burstablePercentage)
		} else {
			info.BurstableCPUPercentage = "N/A (CPU period is zero)"
		}
	}

	// Read cpu.weight as well for v2
	cpuWeightPath := filepath.Join(cgroupV2Mountpoint, cpuWeightV2Filename)
	weightContent, err := ioutil.ReadFile(cpuWeightPath)
	if err != nil {
		info.CPUWeight = fmt.Sprintf("Error reading %s: %v", cpuWeightPath, err)
	} else {
		info.CPUWeight = strings.TrimSpace(string(weightContent))
	}

	return info
}

// getCPUMaxInfoV1 reads CPU limits from cgroup v1 files
func getCPUMaxInfoV1() CgroupInfo {
	info := CgroupInfo{CgroupVersion: "v1"}

	quotaPath := filepath.Join(cgroupV1Mountpoint, cpuQuotaV1Filename)
	periodPath := filepath.Join(cgroupV1Mountpoint, cpuPeriodV1Filename)
	sharesPath := filepath.Join(cgroupV1Mountpoint, cpuSharesV1Filename)

	quotaContent, err := ioutil.ReadFile(quotaPath)
	if err != nil {
		info.Error = fmt.Sprintf("Error reading %s: %v", quotaPath, err)
		return info
	}
	periodContent, err := ioutil.ReadFile(periodPath)
	if err != nil {
		info.Error = fmt.Sprintf("Error reading %s: %v", periodPath, err)
		return info
	}
	sharesContent, err := ioutil.ReadFile(sharesPath)
	if err != nil {
		info.CPUShares = fmt.Sprintf("Error reading %s: %v", sharesPath, err)
	} else {
		info.CPUShares = strings.TrimSpace(string(sharesContent))
	}

	quotaStr := strings.TrimSpace(string(quotaContent))
	periodStr := strings.TrimSpace(string(periodContent))

	period, err := strconv.ParseInt(periodStr, 10, 64)
	if err != nil {
		info.Error = fmt.Sprintf("Error parsing period value from %s: %v", periodPath, err)
		return info
	}
	info.CPUPeriod = fmt.Sprintf("%d microseconds", period)

	quota, err := strconv.ParseInt(quotaStr, 10, 64)
	if err != nil {
		info.Error = fmt.Sprintf("Error parsing quota value from %s: %v", quotaPath, err)
		return info
	}

	if quota == -1 {
		info.CPUMax = "unlimited (no quota)"
		info.BurstableCPUPercentage = "N/A (unlimited)"
	} else {
		info.CPUMax = fmt.Sprintf("%d microseconds", quota)
		if period > 0 {
			burstablePercentage := float64(quota) / float64(period) * 100
			info.BurstableCPUPercentage = fmt.Sprintf("%.2f%%", burstablePercentage)
		} else {
			info.BurstableCPUPercentage = "N/A (CPU period is zero)"
		}
	}

	return info
}

// handler is the HTTP handler function for the root path "/"
func handler(w http.ResponseWriter, r *http.Request) {
	log.Printf("Received request from %s for %s", r.RemoteAddr, r.URL.Path)

	cgroupVersion := detectCgroupVersion()
	var info CgroupInfo

	switch cgroupVersion {
	case "v2":
		info = getCPUMaxInfoV2()
	case "v1":
		info = getCPUMaxInfoV1()
	default:
		info.Error = "Could not detect cgroup version (v1 or v2). Ensure /sys/fs/cgroup is correctly mounted and accessible."
		info.CgroupVersion = "unknown"
	}

	fmt.Fprintf(w, "<html><head><title>Container CPU Info</title>")
	fmt.Fprintf(w, "<style>")
	fmt.Fprintf(w, "body { font-family: Arial, sans-serif; margin: 20px; background-color: #f4f4f4; color: #333; }")
	fmt.Fprintf(w, "div { background-color: #fff; padding: 20px; border-radius: 8px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); max-width: 600px; margin: auto; }")
	fmt.Fprintf(w, "h1 { color: #0056b3; text-align: center; }")
	fmt.Fprintf(w, "p { font-size: 1.1em; line-height: 1.6; }")
	fmt.Fprintf(w, "strong { color: #007bff; }")
	fmt.Fprintf(w, ".error { color: red; font-weight: bold; }")
	fmt.Fprintf(w, "</style>")
	fmt.Fprintf(w, "</head><body>")
	fmt.Fprintf(w, "<div>")
	fmt.Fprintf(w, "<h1>Container CPU Information</h1>")

	if info.Error != "" {
		fmt.Fprintf(w, "<p class=\"error\">Error: %s</p>", info.Error)
	} else {
		fmt.Fprintf(w, "<p><strong>Cgroup Version:</strong> %s</p>", info.CgroupVersion)
		fmt.Fprintf(w, "<p><strong>CPU Max (burst) for container:</strong> %s</p>", info.CPUMax)
		fmt.Fprintf(w, "<p><strong>CPU Period for container:</strong> %s</p>", info.CPUPeriod)
		if info.BurstableCPUPercentage != "" {
			fmt.Fprintf(w, "<p><strong>Burstable CPU Percentage:</strong> %s</p>", info.BurstableCPUPercentage)
		}
		if info.CgroupVersion == "v1" && info.CPUShares != "" {
			fmt.Fprintf(w, "<p><strong>CPU Shares (v1):</strong> %s</p>", info.CPUShares)
		}
		if info.CgroupVersion == "v2" && info.CPUWeight != "" {
			fmt.Fprintf(w, "<p><strong>CPU Weight (v2):</strong> %s</p>", info.CPUWeight)
		}
	}
	fmt.Fprintf(w, "</div>")
	fmt.Fprintf(w, "</body></html>")
}

func main() {
	http.HandleFunc("/", handler) // Register the handler function for the root path
	log.Printf("Server starting on port %s", port)
	log.Fatal(http.ListenAndServe(port, nil)) // Start the HTTP server
}
