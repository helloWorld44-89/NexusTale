output "server_ip" {
  description = "Server public IPv4 address"
  value       = hcloud_server.main.ipv4_address
}

output "server_ipv6" {
  description = "Server public IPv6 address"
  value       = hcloud_server.main.ipv6_address
}

output "data_volume_id" {
  description = "Hetzner volume ID"
  value       = hcloud_volume.data.id
}

output "data_volume_device" {
  description = "Block device path on the server (stable scsi-by-id path)"
  value       = hcloud_volume.data.linux_device
}

output "ansible_run_command" {
  description = "Ansible command to provision + deploy to this server (fill in secrets before running)"
  value       = <<-EOT
    ansible-playbook -i infra/ansible/inventory/prod.ini infra/ansible/deploy-prod.yml \
      -e "prod_vm_host=${hcloud_server.main.ipv4_address}" \
      -e "prod_vm_user=root" \
      -e "workspace=$(pwd)" \
      -e "nexustale_domain=${var.domain}" \
      -e "data_volume_device=${hcloud_volume.data.linux_device}" \
      -e "image_tag=REPLACE_WITH_IMAGE_TAG" \
      -e "ghcr_token=REPLACE_WITH_GHCR_PAT" \
      -e "nexustale_db_password=REPLACE_WITH_DB_PASS" \
      -e "nexustale_jwt_secret=REPLACE_WITH_JWT_SECRET" \
      -e "nexustale_encryption_key=$(openssl rand -hex 32)" \
      -e "nexustale_minio_accesskey=REPLACE_WITH_MINIO_USER" \
      -e "nexustale_minio_secretkey=REPLACE_WITH_MINIO_PASS" \
      -e "nexustale_alert_email=REPLACE_WITH_EMAIL"
  EOT
}
