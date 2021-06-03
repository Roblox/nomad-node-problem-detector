job "aggregator" {
  datacenters = ["dc1"]
  type = "service"

  group "aggregator-group" {
    task "aggregator-task" {
      driver = "docker"

      config {
	network_mode = "host"
	image = "shm32/npd-aggregator:1.0.4"
	command = "npd"
	args    = ["aggregator", "--aggregation-cycle-time=3s"]
      }

      resources {
        cpu    = 500
        memory = 256
      }
    }
  }
}
