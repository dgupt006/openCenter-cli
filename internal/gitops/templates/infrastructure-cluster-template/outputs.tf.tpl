# The CLI lifecycle contract is opt-in. Legacy mode keeps these outputs
# compatible with Kubespray modules that predate the contract.
output "opencenter_kubespray_inventory_path" {
  value = try(module.kubespray-cluster.inventory_path, null)
}

output "opencenter_kubespray_lifecycle_contract_version" {
  value = try(module.kubespray-cluster.lifecycle_contract_version, null)

  precondition {
    condition = var.opencenter_lifecycle_mode == "legacy" || try(
      tostring(module.kubespray-cluster.lifecycle_contract_version),
      "",
    ) == "2"
    error_message = "CLI lifecycle mode requires Kubespray lifecycle contract version 2 (inventory_output_dir support). Regenerate the infrastructure tree with 'opencenter cluster generate' and update the kubespray module source ref."
  }
}

output "opencenter_kubespray_api_address" {
  value = try(module.kubespray-cluster.k8s_api_address, null)
}

output "opencenter_kubespray_api_port" {
  value = try(module.kubespray-cluster.k8s_api_port, null)
}
