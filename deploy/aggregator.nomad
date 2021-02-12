job "aggregator" {
  datacenters = ["dc1"]
  type = "service"

  group "aggregator-group" {
    network {
      mode = bridge
    }

    task "aggregator-task" {
      driver = "docker"

      config {
	image = "shm32/npd-aggregator:1.0.0"
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
