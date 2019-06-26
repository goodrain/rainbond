#!/bin/bash
set -e
env2file conversion -f /root/envoy_config.yaml
cluster_name=${TENANT_ID}_${PLUGIN_ID}_${SERVICE_NAME}
exec envoy -c /root/envoy_config.yaml --service-cluster ${cluster_name} --service-node ${cluster_name}