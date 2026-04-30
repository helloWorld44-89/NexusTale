variable "hcloud_token" {
  description = "Hetzner Cloud API token (generate at console.hetzner.cloud → Security → API Tokens)"
  type        = string
  sensitive   = true
}

variable "ssh_public_key" {
  description = "SSH public key to install on the server"
  type        = string
  sensitive   = true
}

variable "env" {
  description = "Environment label applied to all resources (e.g. prod, staging)"
  type        = string
  default     = "prod"
}

variable "server_type" {
  description = <<-EOT
    Hetzner server type. US datacenters (ash, hil) have a smaller selection
    than EU — check https://www.hetzner.com/cloud for what's currently
    available in your chosen location.
    Recommended minimum for NexusTale: 4 vCPU / 8 GB RAM.
  EOT
  type        = string
  # No default — pick from the current Hetzner catalog after checking availability.
}

variable "location" {
  description = <<-EOT
    Hetzner datacenter. US options:
      ash — Ashburn, Virginia (east coast)
      hil — Hillsboro, Oregon (west coast)
  EOT
  type        = string
  default     = "ash"
}

variable "domain" {
  description = "Public domain name for the NexusTale instance (e.g. app.nexustale.io)"
  type        = string
}

variable "data_volume_size_gb" {
  description = "Size of the persistent data volume in GB (stores Docker volumes, backups, git repos)"
  type        = number
  default     = 80
}
