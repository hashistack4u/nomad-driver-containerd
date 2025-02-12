log_level = "INFO"
data_dir = "/tmp/nomad"

advertise {
  http = "127.0.0.1"
  rpc  = "127.0.0.1"
  serf = "127.0.0.1"
}

plugin "containerd-driver" {
  config {
    enabled = true
    containerd_runtime = "io.containerd.runc.v2"
    stats_interval = "5s"
    allow_privileged = true
  }
}

server {
  enabled = true
  bootstrap_expect = 1
  default_scheduler_config {
    scheduler_algorithm = "spread"
    memory_oversubscription_enabled = true

    preemption_config {
      batch_scheduler_enabled   = true
      system_scheduler_enabled  = true
      service_scheduler_enabled = true
    }
  }
}

client {
  enabled = true
  host_volume "s1" {
    path = "/tmp/host_volume/s1"
    read_only = false
  }
  options {
    "driver.allowlist" = "containerd-driver"
  }
}
