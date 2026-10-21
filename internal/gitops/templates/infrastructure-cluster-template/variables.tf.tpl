variable "opencenter_lifecycle_mode" {
  type    = string
  default = "legacy"

  validation {
    condition     = contains(["legacy", "cli"], var.opencenter_lifecycle_mode)
    error_message = "opencenter_lifecycle_mode must be either legacy or cli."
  }
}

variable "inventory_output_dir" {
  type        = string
  default     = ""
  description = "Directory the Kubespray inventory subtree renders into. The CLI passes its state-zone inventory directory with opencenter_lifecycle_mode=cli so the worktree stays clean; legacy mode keeps the module default of ./inventory under the working directory."
}

variable "os_application_credential_id" {
  type    = string
  default = ""
}

variable "os_application_credential_secret" {
  type    = string
  default = ""
}

variable "openstack_admin_name" {
  type    = string
  default = "admin"
}

variable "openstack_admin_password" {
  type    = string
  default = ""
}

variable "openstack_user_name" {
  type    = string
  default = ""
}

variable "openstack_user_password" {
  type    = string
  default = ""
}

variable "pf9_account_url" {
  type    = string
  default = ""
}

variable "pf9_username" {
  type    = string
  default = ""
}

variable "pf9_password" {
  type    = string
  default = ""
}

variable "worker_count" {
  type    = string
  default = "1"

}

variable "master_count" {
  type    = string
  default = "3"
}

variable "windows_admin_password" {
  type    = string
  default = ""
}
