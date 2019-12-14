variable aks_client_id {}
variable aks_client_secret {}
variable aks_resource_group_name {}

variable aks_open_port {
  default = 22
}

variable "aks_worker_count" {
  default = 1
}

variable "aks_worker_type" {
  default = "Standard_DS1_v2"
}

variable "ssh_public_key" {
  default = "~/.ssh/id_rsa.pub"
}

variable "dns_prefix" {
  default = "k8stest"
}

variable cluster_name {
  default = "k8stest"
}

variable location {
  default = "West US 2"
}
