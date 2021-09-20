job "aggregator" {
  datacenters = ["dc1"]
  type = "service"

  group "aggregator-group" {
    task "aggregator-task" {
      driver = "docker"

      config {
	network_mode = "host"
	image = "shm32/npd-aggregator:1.0.9"
	command = "npd"
	args    = ["aggregator"]
      }

      resources {
        cpu    = 500
        memory = 256
      }
    }
  }
}
