terraform {
  required_version = ">= 0.12.0"
}

provider "azurerm" {
  version = "~>1.5"
}

provider "external" {
  version = "~> 1.2"
}

resource "azurerm_resource_group" "k8s" {
  name     = var.aks_resource_group_name
  location = var.location
}

# get the auto-generated NSG name
data "external" "aks_nsg_id" {
  program = [
    "bash",
    "aks_nsg_id"
  ]
  depends_on = [azurerm_kubernetes_cluster.k8s]
}

# get the auto-generated resource group name
data "external" "aks_rg_name" {
  program = [
    "bash",
    "aks_rg_name"
  ]
  depends_on = [azurerm_kubernetes_cluster.k8s]
}

resource "azurerm_network_security_rule" "openport" {
  name                       = "openport"
  priority                   = 1001
  direction                  = "Inbound"
  access                     = "Allow"
  protocol                   = "Tcp"
  source_port_range          = "*"
  destination_port_range     = var.aks_open_port
  source_address_prefix      = "*"
  destination_address_prefix = "*"
  resource_group_name         = data.external.aks_rg_name.result.output
  network_security_group_name = data.external.aks_nsg_id.result.output
}

resource "azurerm_kubernetes_cluster" "k8s" {
  name                = var.cluster_name
  location            = azurerm_resource_group.k8s.location
  resource_group_name = azurerm_resource_group.k8s.name
  dns_prefix          = var.dns_prefix

  linux_profile {
    admin_username = "azureuser"

    ssh_key {
      key_data = file(var.ssh_public_key)
    }
  }

  default_node_pool {
    name            = "agentpool"
    node_count      = var.aks_worker_count
    vm_size         = var.aks_worker_type
    enable_node_public_ip = true
  }

  network_profile {
    network_plugin = "azure"
  }

  service_principal {
    client_id     = var.aks_client_id
    client_secret = var.aks_client_secret
  }

  tags = {
    Environment = "Development"
  }
}