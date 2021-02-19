job "detector" {
  datacenters = ["dc1"]
  type = "system"

  group "detector-group" {
    network {
      port "http" {
        to = 8083
      }
    }

    task "unpack-nnpd" {
      lifecycle {
        hook = "prestart"
        sidecar = false
      }

      driver = "docker"
        config {
          image = "shm32/npd-detector:1.0.7"
        }
      }

    task "detector-task" {
      driver = "raw_exec"

      config {
	command = "npd"
	args    = ["detector"]
      }

      resources {
        cpu    = 500
        memory = 256
      }
    }
  }
}
