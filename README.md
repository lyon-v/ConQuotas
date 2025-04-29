# Containerd RootFS Quota Manager

**A lightweight, XFS-based solution for managing user container rootfs quotas in Kubernetes clusters with Containerd.**

[![License](https://img.shields.io/badge/License-Apache 2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Go](https://img.shields.io/badge/Go-1.18+-00ADD8.svg)](https://golang.org/)
[![Containerd](https://gitee.com/WuLiang2017/imgs/raw/master/img/Containerd-1.6+-green.svg)](https://containerd.io/)
[![XFS](https://img.shields.io/badge/Filesystem-XFS-blue.svg)](https://xfs.org/)

## Overview

The **Containerd RootFS Quota Manager** is a service that enforces storage quotas on user container rootfs in Kubernetes clusters using Containerd as the CRI runtime. Built specifically for XFS filesystems, it leverages XFS project quotas (`pquota`) and Containerd's gRPC API to dynamically manage container storage limits. This ensures efficient disk usage, system stability, and optimized image management.

This project addresses Containerd's lack of native rootfs quota support by utilizing XFS's robust quota capabilities, replacing Docker-based solutions and enabling CRI-compliant features like lazy loading and P2P image distribution.

## Why XFS?

The solution relies on the XFS filesystem's project quota (`pquota`) feature to enforce precise, filesystem-level storage limits for container rootfs. Key benefits include:

- **Granular Control**: XFS project quotas allow per-container storage limits, preventing disk exhaustion.
- **Performance**: XFS is optimized for high-performance workloads, ideal for containerized environments.
- **Scalability**: Supports large-scale Kubernetes clusters with thousands of containers.
- **Compatibility**: Seamlessly integrates with Containerd's OverlayFS storage driver.

## Features

- **XFS Quota Management**: Uses XFS project quotas to enforce rootfs limits (e.g., 10GB default) on container creation and cleanup on deletion.
- **Containerd Integration**: Listens to Containerd gRPC events (TaskCreate, TaskDelete) for real-time quota management.
- **Dynamic Project IDs**: Allocates and recycles XFS project IDs for each container, ensuring concurrent safety.
- **High Availability**: Implements exponential backoff reconnection and persistent state for robust operation.
- **Configurable**: JSON configuration for quota sizes, XFS project ID ranges, and Containerd settings.
- **Systemd Integration**: Deploys as a systemd service with auto-restart and Containerd dependency.
- **Low Maintenance**: Decoupled design avoids Containerd source changes, ensuring future compatibility.

## Benefits

- **Stability**: Prevents disk exhaustion on nodes by enforcing XFS-based quotas, ensuring reliable scheduling.
- **Efficiency**: Limits rootfs growth to reduce image sizes, lowering storage and distribution costs.
- **Future-Ready**: Enables CRI features like lazy loading and P2P image distribution.
- **Ease of Deployment**: Automated setup with tools like Ansible, plus XFS-specific validation.

## Prerequisites

- **XFS Filesystem**: Nodes must use XFS with project quota (`pquota`) enabled. See [XFS Setup](https://grok.com/chat/5d1344e9-32cf-4a57-addd-5338d5a5e955#xfs-setup).
- **Containerd**: Version 1.6 or higher.
- **Kubernetes**: Configured with Containerd as the CRI runtime.
- **Go**: Version 1.18 or higher for building the service.
- **Systemd**: For service deployment.
- **Dependencies**: Access to Containerd gRPC endpoint and XFS quota tools (`xfs_quota`).

## XFS Setup

1. **Format Disk with XFS**:

   ```bash
   mkfs.xfs -f /dev/sdX
   ```

2. **Mount with pquota**:

   - Edit 

     ```
     /etc/fstab
     ```

      or mount manually:

     ```bash
     mount -o pquota /dev/sdX /path/to/mount
     ```

   - Verify:

     ```bash
     mount | grep pquota
     ```

3. **Enable Project Quotas**:

   - Initialize project quota support:

     ```bash
     xfs_quota -x -c 'project -setup -p /path/to/mount 1000' /path/to/mount
     ```

   - Confirm quota functionality:

     ```bash
     xfs_quota -x -c 'report -p' /path/to/mount
     ```

## Installation

1. **Prepare Environment**:

   - Ensure nodes have Containerd and XFS with `pquota` enabled (see [XFS Setup](https://grok.com/chat/5d1344e9-32cf-4a57-addd-5338d5a5e955#xfs-setup)).
   - Verify Containerd gRPC endpoint connectivity.

2. **Clone Repository**:

   ```bash
   git clone https://github.com/your-org/containerd-rootfs-quota.git
   cd containerd-rootfs-quota
   ```

3. **Build the Service**:

   ```bash
   go build -o containerd-quota-manager ./cmd/quota-manager
   ```

4. **Configure**:

   - Create 

     ```
     /etc/containerd-quota/config.json
     ```

     :

     ```json
     {
       "containerd_grpc_addr": "unix:///run/containerd/containerd.sock",
       "namespace": "k8s.io",
       "state_file": "/var/lib/containerd-quota/state.json",
       "project_id_range": [1000, 10000],
       "default_quota_mb": 100
     }
     ```

   - Ensure 

     ```
     /var/lib/containerd-quota
     ```

      exists:

     ```bash
     sudo mkdir -p /var/lib/containerd-quota
     ```

5. **Deploy with Systemd**:

   - Copy binary:

     ```bash
     sudo cp containerd-quota-manager /usr/local/bin/
     ```

   - Install systemd service:

     ```bash
     sudo cp deploy/containerd-quota.service /etc/systemd/system/
     sudo systemctl enable containerd-quota
     sudo systemctl start containerd-quota
     ```

6. **Verify**:

   - Check service status:

     ```bash
     sudo systemctl status containerd-quota
     ```

   - Create a container and verify XFS quota:

     ```bash
     xfs_quota -x -c 'report -p' /path/to/xfs/mount
     ```

## Usage

The service automatically:

- Monitors Containerd TaskCreate/TaskDelete events via gRPC.
- Assigns an XFS project ID and applies a quota (default: 100MB) to the container's rootfs `upperdir` (OverlayFS).
- Removes the quota and recycles the project ID on deletion.
- Persists state in `/var/lib/containerd-quota/state.json` for restart recovery.

View logs for debugging:

```bash
journalctl -u containerd-quota
```

## Testing

1. **Unit Tests**:

   ```bash
   go test ./...
   ```

2. **Integration Tests**:

   - Deploy on a test Kubernetes cluster with XFS.

   - Create/delete containers and verify quotas:

     ```bash
     xfs_quota -x -c 'report -p' /path/to/xfs/mount
     ```

3. **Stress Test**:

   - Simulate high container churn to validate concurrent safety and XFS quota performance.

## Roadmap

- **Phase 1** (Completed):
  - CRI switch to Containerd with XFS pquota enabled.
  - XFS-based rootfs quota service development and testing.
  - User image submission compatibility with Containerd.
- **Phase 2** (Planned):
  - Image lazy loading (e.g., stargz snapshotter) for faster startup.
  - P2P image distribution (e.g., Dragonfly) for efficient pulling.

## Contributing

We welcome contributions! Please:

1. Fork the repository.
2. Create a feature branch (`git checkout -b feature/my-feature`).
3. Commit changes (`git commit -m "Add my feature"`).
4. Push to the branch (`git push origin feature/my-feature`).
5. Open a pull request.

See [CONTRIBUTING.md](https://grok.com/chat/CONTRIBUTING.md) for details.

## License

Licensed under the Apache License 2.0. See [LICENSE](https://grok.com/chat/LICENSE).

## Contact

For issues or questions, file a GitHub issue or email [support@containerd-quota.org](mailto:support@containerd-quota.org).

------

**Optimize your Kubernetes clusters with XFS-powered container quotas!**