package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	cgroupV1Root = "/sys/fs/cgroup"
	cgroupV2Root = "/sys/fs/cgroup" // cgroup v2 uses a unified hierarchy under this root
	samplePeriod = 2 * time.Second  // Time to wait between CPU usage readings
)

// CgroupVersion represents the detected cgroup version
type CgroupVersion int

const (
	UnknownCgroup CgroupVersion = iota
	CgroupV1
	CgroupV2
)

func (cv CgroupVersion) String() string {
	switch cv {
	case CgroupV1:
		return "cgroup v1"
	case CgroupV2:
		return "cgroup v2"
	default:
		return "unknown cgroup version"
	}
}

// detectCgroupVersion tries to determine if the system is using cgroup v1 or v2
func detectCgroupVersion() CgroupVersion {
	// Check for cgroup v2 unified hierarchy indicator
	if _, err := os.Stat(filepath.Join(cgroupV2Root, "cgroup.controllers")); err == nil {
		return CgroupV2
	}
	// Check for cgroup v1 specific controllers (e.g., cpu, memory)
	if _, err := os.Stat(filepath.Join(cgroupV1Root, "cpu")); err == nil {
		return CgroupV1
	}
	return UnknownCgroup
}

// readCgroupFile reads the content of a cgroup file and returns it as a string
func readCgroupFile(path string) (string, error) {
	content, err := ioutil.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read cgroup file %s: %w", path, err)
	}
	return strings.TrimSpace(string(content)), nil
}

// parseCgroupLine parses a line from /proc/self/cgroup and returns the controller and path
func parseCgroupLine(line string) (string, string) {
	parts := strings.SplitN(line, ":", 3)
	if len(parts) != 3 {
		return "", ""
	}
	return parts[1], parts[2] // controllers, path
}

// bytesToMiB converts bytes to MiB
func bytesToMiB(bytes uint64) float64 {
	return float64(bytes) / 1024 / 1024
}

// getCPUUsageV1 reads cumulative CPU usage from cpuacct.usage (nanoseconds)
func getCPUUsageV1(cpuPath string) (uint64, error) {
	fullPath := filepath.Join(cgroupV1Root, "cpu", cpuPath, "cpuacct.usage")
	usageStr, err := readCgroupFile(fullPath)
	if err != nil {
		return 0, err
	}
	usage, err := strconv.ParseUint(usageStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse CPU usage from %s: %w", fullPath, err)
	}
	return usage, nil
}

// getCPUUsageV2 reads cumulative CPU usage from cpu.stat (microseconds)
func getCPUUsageV2(unifiedPath string) (uint64, error) {
	fullPath := filepath.Join(cgroupV2Root, unifiedPath, "cpu.stat")
	statContent, err := readCgroupFile(fullPath)
	if err != nil {
		return 0, err
	}
	scanner := bufio.NewScanner(strings.NewReader(statContent))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "usage_usec") {
			parts := strings.Fields(line)
			if len(parts) == 2 {
				usage, err := strconv.ParseUint(parts[1], 10, 64)
				if err != nil {
					return 0, fmt.Errorf("failed to parse usage_usec from %s: %w", fullPath, err)
				}
				return usage, nil
			}
		}
	}
	return 0, fmt.Errorf("usage_usec not found in cpu.stat at %s", fullPath)
}

func main() {
	fmt.Println("--- Cgroup Information ---")

	cgroupVer := detectCgroupVersion()
	fmt.Printf("Detected Cgroup Version: %s\n", cgroupVer)

	if cgroupVer == UnknownCgroup {
		fmt.Println("Cannot determine cgroup version. Exiting.")
		return
	}

	// Read /proc/self/cgroup to get the cgroup paths for the current process
	f, err := os.Open("/proc/self/cgroup")
	if err != nil {
		fmt.Printf("Error opening /proc/self/cgroup: %v\n", err)
		fmt.Println("This program needs to run in a Linux environment with cgroups enabled.")
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	cgroupPaths := make(map[string]string) // map of controller to relative path

	fmt.Println("\n--- /proc/self/cgroup Content ---")
	for scanner.Scan() {
		line := scanner.Text()
		fmt.Println(line)
		controllers, path := parseCgroupLine(line)
		if cgroupVer == CgroupV1 {
			if strings.Contains(controllers, "cpu") {
				cgroupPaths["cpu"] = path
			}
			if strings.Contains(controllers, "memory") {
				cgroupPaths["memory"] = path
			}
		} else if cgroupVer == CgroupV2 {
			if _, ok := cgroupPaths["unified"]; !ok && path != "/" {
				cgroupPaths["unified"] = path
			}
		}
	}
	if err := scanner.Err(); err != nil {
		fmt.Printf("Error reading /proc/self/cgroup: %v\n", err)
	}

	fmt.Println("\n--- Resource Limits and Usage ---")

	var cpuLimitCores float64 = -1.0 // Initialize with -1 to indicate no limit found

	if cgroupVer == CgroupV1 {
		if cpuPath, ok := cgroupPaths["cpu"]; ok {
			fmt.Println("\nCPU (cgroup v1):")
			fullPath := filepath.Join(cgroupV1Root, "cpu", cpuPath)
			if quotaStr, err := readCgroupFile(filepath.Join(fullPath, "cpu.cfs_quota_us")); err == nil {
				if val, err := strconv.ParseInt(quotaStr, 10, 64); err == nil {
					if val != -1 {
						if periodStr, err := readCgroupFile(filepath.Join(fullPath, "cpu.cfs_period_us")); err == nil {
							if period, err := strconv.ParseInt(periodStr, 10, 64); err == nil && period > 0 {
								cpuLimitCores = float64(val) / float64(period)
								fmt.Printf("  CPU Quota (microseconds): %s\n", quotaStr)
								fmt.Printf("  CPU Period (microseconds): %s\n", periodStr)
								fmt.Printf("  Equivalent CPU Cores Limit: %.2f\n", cpuLimitCores)
							}
						}
					} else {
						fmt.Printf("  CPU Quota: %s (No explicit limit)\n", quotaStr)
					}
				}
			}

			// Calculate CPU Utilization
			usage1, err1 := getCPUUsageV1(cpuPath)
			if err1 != nil {
				fmt.Printf("  Error getting initial CPU usage: %v\n", err1)
			} else {
				fmt.Printf("  Initial CPU Usage (cumulative nanoseconds): %d\n", usage1)
				fmt.Printf("  Sampling CPU utilization for %s...\n", samplePeriod)
				time.Sleep(samplePeriod)
				usage2, err2 := getCPUUsageV1(cpuPath)
				if err2 != nil {
					fmt.Printf("  Error getting second CPU usage: %v\n", err2)
				} else {
					cpuDelta := float64(usage2 - usage1)             // in nanoseconds
					timeDelta := float64(samplePeriod.Nanoseconds()) // in nanoseconds

					// CPU usage in cores = (CPU_delta_nanoseconds / time_delta_nanoseconds)
					actualCPUUsageCores := cpuDelta / timeDelta
					fmt.Printf("  Current CPU Usage (cores): %.4f\n", actualCPUUsageCores)

					if cpuLimitCores > 0 {
						utilization := (actualCPUUsageCores / cpuLimitCores) * 100
						fmt.Printf("  CPU Utilization of Limit: %.2f%%\n", utilization)
					} else {
						fmt.Println("  Cannot calculate CPU Utilization of Limit: CPU limit not found or is unlimited.")
					}
				}
			}

			if stat, err := readCgroupFile(filepath.Join(fullPath, "cpu.stat")); err == nil {
				fmt.Printf("  CPU Stat:\n%s\n", indent(stat, "    "))
			}
		} else {
			fmt.Println("CPU cgroup path not found for v1.")
		}

		if memPath, ok := cgroupPaths["memory"]; ok {
			fmt.Println("\nMemory (cgroup v1):")
			fullPath := filepath.Join(cgroupV1Root, "memory", memPath)
			if limit, err := readCgroupFile(filepath.Join(fullPath, "memory.limit_in_bytes")); err == nil {
				if val, _ := strconv.ParseUint(limit, 10, 64); val > 0 && val != 9223372036854771712 { // 9223372036854771712 is common for no limit
					fmt.Printf("  Memory Limit: %s bytes (%.2f MiB)\n", limit, bytesToMiB(val))
				} else {
					fmt.Printf("  Memory Limit: %s bytes (No explicit limit or very large)\n", limit)
				}
			}
			if usage, err := readCgroupFile(filepath.Join(fullPath, "memory.usage_in_bytes")); err == nil {
				if val, _ := strconv.ParseUint(usage, 10, 64); val > 0 {
					fmt.Printf("  Memory Usage: %s bytes (%.2f MiB)\n", usage, bytesToMiB(val))
				}
			}
			if stat, err := readCgroupFile(filepath.Join(fullPath, "memory.stat")); err == nil {
				fmt.Printf("  Memory Stat:\n%s\n", indent(stat, "    "))
			}
		} else {
			fmt.Println("Memory cgroup path not found for v1.")
		}

	} else if cgroupVer == CgroupV2 {
		if unifiedPath, ok := cgroupPaths["unified"]; ok {
			fmt.Println("\nCPU (cgroup v2):")
			fullPath := filepath.Join(cgroupV2Root, unifiedPath)
			if max, err := readCgroupFile(filepath.Join(fullPath, "cpu.max")); err == nil {
				parts := strings.Fields(max)
				if len(parts) == 2 {
					quota, _ := strconv.ParseInt(parts[0], 10, 64)
					period, _ := strconv.ParseInt(parts[1], 10, 64)
					fmt.Printf("  CPU Max (quota period): %s (quota: %s us, period: %s us)\n", max, parts[0], parts[1])
					if quota != -1 && period > 0 {
						cpuLimitCores = float64(quota) / float64(period)
						fmt.Printf("  Equivalent CPU Cores Limit: %.2f\n", cpuLimitCores)
					}
				} else {
					fmt.Printf("  CPU Max: %s\n", max)
				}
			}

			// Calculate CPU Utilization
			usage1, err1 := getCPUUsageV2(unifiedPath)
			if err1 != nil {
				fmt.Printf("  Error getting initial CPU usage: %v\n", err1)
			} else {
				fmt.Printf("  Initial CPU Usage (cumulative microseconds): %d\n", usage1)
				fmt.Printf("  Sampling CPU utilization for %s...\n", samplePeriod)
				time.Sleep(samplePeriod)
				usage2, err2 := getCPUUsageV2(unifiedPath)
				if err2 != nil {
					fmt.Printf("  Error getting second CPU usage: %v\n", err2)
				} else {
					cpuDelta := float64(usage2 - usage1)              // in microseconds
					timeDelta := float64(samplePeriod.Microseconds()) // in microseconds

					// CPU usage in cores = (CPU_delta_microseconds / time_delta_microseconds)
					actualCPUUsageCores := cpuDelta / timeDelta
					fmt.Printf("  Current CPU Usage (cores): %.4f\n", actualCPUUsageCores)

					if cpuLimitCores > 0 {
						utilization := (actualCPUUsageCores / cpuLimitCores) * 100
						fmt.Printf("  CPU Utilization of Limit: %.2f%%\n", utilization)
					} else {
						fmt.Println("  Cannot calculate CPU Utilization of Limit: CPU limit not found or is unlimited.")
					}
				}
			}

			if stat, err := readCgroupFile(filepath.Join(fullPath, "cpu.stat")); err == nil {
				fmt.Printf("  CPU Stat:\n%s\n", indent(stat, "    "))
			}

			fmt.Println("\nMemory (cgroup v2):")
			if max, err := readCgroupFile(filepath.Join(fullPath, "memory.max")); err == nil {
				if val, _ := strconv.ParseUint(max, 10, 64); val > 0 && max != "max" {
					fmt.Printf("  Memory Limit: %s bytes (%.2f MiB)\n", max, bytesToMiB(val))
				} else {
					fmt.Printf("  Memory Limit: %s (No explicit limit or very large)\n", max)
				}
			}
			if current, err := readCgroupFile(filepath.Join(fullPath, "memory.current")); err == nil {
				if val, _ := strconv.ParseUint(current, 10, 64); val > 0 {
					fmt.Printf("  Memory Usage: %s bytes (%.2f MiB)\n", current, bytesToMiB(val))
				}
			}
			if stat, err := readCgroupFile(filepath.Join(fullPath, "memory.stat")); err == nil {
				fmt.Printf("  Memory Stat:\n%s\n", indent(stat, "    "))
			}
		} else {
			fmt.Println("Unified cgroup path not found for v2.")
		}
	}
}

// indent adds a prefix to each line of a string
func indent(s, prefix string) string {
	lines := strings.Split(s, "\n")
	for i := range lines {
		lines[i] = prefix + lines[i]
	}
	return strings.Join(lines, "\n")
}
