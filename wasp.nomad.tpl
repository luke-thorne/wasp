variable "wasp_config" {
	default = <<EOH
{
	"database": {
		"directory": "{{ env "NOMAD_TASK_DIR" }}/waspdb"
	},
	"logger": {
		"level": "debug",
		"disableCaller": false,
		"disableStacktrace": true,
		"encoding": "console",
		"outputPaths": [
			"stdout",
			"wasp.log"
		],
		"disableEvents": true
	},
	"network": {
		"bindAddress": "0.0.0.0",
		"externalAddress": "auto"
	},
	"node": {
		"disablePlugins": [],
		"enablePlugins": ["metrics"]
	},
	"webapi": {
		"bindAddress": "0.0.0.0:{{ env "NOMAD_PORT_api" }}",
		"adminWhitelist": ${adminWhitelist}
	},
	"metrics": {
        "bindAddress": "0.0.0.0:{{ env "NOMAD_PORT_metrics" }}",
        "enabled": true
	},
	"dashboard": {
		"auth": {
			"scheme": "basic",
			"username": "wasp",
			"password": "wasp"
		},
		"bindAddress": "0.0.0.0:{{ env "NOMAD_PORT_dashboard" }}"
	},
	"peering":{
		"port": {{ env "NOMAD_PORT_peering" }},
		"netid": "{{ env "NOMAD_ADDR_peering" }}"
	},
	"nodeconn": {
		"address": "{{ range service "goshimmer-leader-node" }}{{ if in .Tags "txstream" }}{{ .Address }}:{{ .Port }}{{end}}{{end}}"
	},
	"nanomsg":{
		"port": {{ env "NOMAD_PORT_nanomsg" }}
	}
}
EOH
}

job "iscp-evm" {
	datacenters = ["hcloud"]

	update {
		max_parallel      = 1
		health_check      = "task_states"
		min_healthy_time  = "1s"
		healthy_deadline  = "30s"
		progress_deadline = "5m"
		auto_revert       = true
		auto_promote      = true
		canary            = 1
		stagger           = "15s"
	}

	group "node" {
		ephemeral_disk {
			migrate = true
			sticky = true
		}

		count = 5

		network {
			mode = "host"

			port "dashboard" {
				host_network = "private"
			}
			port "api" {
				host_network = "private"
			}
			port "nanomsg" {
				host_network = "private"
			}
			port "peering" {
				host_network = "private"
			}
			port "metrics" {
				host_network = "private"
			}
		}

		task "wasp" {
			driver = "docker"

			config {
				network_mode = "host"
				image = "${artifact.image}:${artifact.tag}"
				entrypoint = ["wasp", "-c", "/local/config.json"]
				ports = [
					"dashboard",
					"api",
					"nanomsg",
					"peering",
					"metrics",
				]

				labels = {
					"co.elastic.metrics/raw" = "[{\"metricsets\":[\"collector\"],\"module\":\"prometheus\",\"period\":\"10s\",\"metrics_path\":\"/metrics\",\"hosts\":[\"$${NOMAD_ADDR_metrics}\"]}]"
				}

				auth {
					username = "${auth.username}"
					password = "${auth.password}"
					server_address = "${auth.server_address}"
				}
			}

			service {
				tags = ["wasp", "api"]
				port  = "api"
				check {
					type     = "http"
					port     = "api"
					path     = "info"
					interval = "5s"
					timeout  = "2s"
				}
			}
			service {
				tags = ["wasp", "dashboard"]
				port  = "dashboard"
			}
			service {
				tags = ["wasp", "nanomsg"]
				port  = "nanomsg"
			}
			service {
				tags = ["wasp", "peering"]
				port  = "peering"
			}
			service {
				tags = ["wasp", "metrics"]
				port  = "metrics"
			}

			template {
				data = var.wasp_config
				destination = "/local/config.json"
				perms = "777"
			}

			resources {
				memory = 3000
				cpu = 2000
			}
		}
	}

	group "access" {
		ephemeral_disk {
			migrate = true
			sticky = true
		}

		count = 3

		network {
			mode = "host"

			port "dashboard" {
				host_network = "private"
			}
			port "api" {
				host_network = "private"
			}
			port "nanomsg" {
				host_network = "private"
			}
			port "peering" {
				host_network = "private"
			}
			port "metrics" {
				host_network = "private"
			}
		}

		task "wasp" {
			driver = "docker"

			config {
				network_mode = "host"
				image = "${artifact.image}:${artifact.tag}"
				command = "wasp"
				entrypoint = [""]
				args = [
					"-c=/local/config.json",
				]
				ports = [
					"dashboard",
					"api",
					"nanomsg",
					"peering",
					"metrics",
				]

				labels = {
					"co.elastic.metrics/module" = "prometheus"
					"co.elastic.metrics/hosts" = "$${NOMAD_ADDR_metrics}"
					"co.elastic.metrics/path" = "/metrics"
					"co.elastic.metrics/period" = "/10s"
				}

				auth {
					username = "${auth.username}"
					password = "${auth.password}"
					server_address = "${auth.server_address}"
				}
			}

			service {
				tags = ["wasp", "api"]
				port  = "api"
				check {
					type     = "http"
					port     = "api"
					path     = "info"
					interval = "5s"
					timeout  = "2s"
				}
			}
			service {
				tags = ["wasp", "dashboard"]
				port  = "dashboard"
			}
			service {
				tags = ["wasp", "nanomsg"]
				port  = "nanomsg"
			}
			service {
				tags = ["wasp", "peering"]
				port  = "peering"
			}
			service {
				tags = ["wasp", "metrics"]
				port  = "metrics"
			}

			template {
				data = var.wasp_config
				destination = "/local/config.json"
				perms = "777"
			}

			resources {
				memory = 3000
				cpu = 2000
			}
		}
	}
}
