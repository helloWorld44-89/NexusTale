terraform {
  required_version = ">= 1.6"
  required_providers {
    hcloud = {
      source  = "hetznercloud/hcloud"
      version = "~> 1.49"
    }
  }
}

provider "hcloud" {
  token = var.hcloud_token
}

# ── SSH Key ─────────────────────────────────────────────────────────────────

resource "hcloud_ssh_key" "nexustale" {
  name       = "${var.env}-nexustale-key"
  public_key = var.ssh_public_key
}

# ── Firewall ────────────────────────────────────────────────────────────────
# Only 22/80/443 and ICMP inbound; everything else blocked.
# MinIO (9000/9001) and the API port (8080) are NOT opened here —
# they are bound to 127.0.0.1 inside the host via docker-compose.prod-override.yml.

resource "hcloud_firewall" "nexustale" {
  name = "${var.env}-nexustale-fw"

  rule {
    direction  = "in"
    protocol   = "tcp"
    port       = "22"
    source_ips = ["0.0.0.0/0", "::/0"]
  }

  rule {
    direction  = "in"
    protocol   = "tcp"
    port       = "80"
    source_ips = ["0.0.0.0/0", "::/0"]
  }

  rule {
    direction  = "in"
    protocol   = "tcp"
    port       = "443"
    source_ips = ["0.0.0.0/0", "::/0"]
  }

  rule {
    direction  = "in"
    protocol   = "icmp"
    source_ips = ["0.0.0.0/0", "::/0"]
  }
}

# ── Server ───────────────────────────────────────────────────────────────────

resource "hcloud_server" "main" {
  name         = "${var.env}-nexustale"
  image        = "ubuntu-24.04"
  server_type  = var.server_type
  location     = var.location
  ssh_keys     = [hcloud_ssh_key.nexustale.id]
  firewall_ids = [hcloud_firewall.nexustale.id]

  labels = {
    env = var.env
    app = "nexustale"
  }
}

# ── Data Volume ──────────────────────────────────────────────────────────────
# Stores Docker named volumes (Postgres, Redis, MinIO, git repos) and backups.
# Terraform pre-formats with ext4; Ansible handles mounting + fstab.
# automount=false so Ansible controls the exact mount point and options.

resource "hcloud_volume" "data" {
  name     = "${var.env}-nexustale-data"
  size     = var.data_volume_size_gb
  location = var.location
  format   = "ext4"

  labels = {
    env = var.env
    app = "nexustale"
  }
}

resource "hcloud_volume_attachment" "data" {
  volume_id = hcloud_volume.data.id
  server_id = hcloud_server.main.id
  automount = false
}
