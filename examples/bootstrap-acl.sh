ACL_TOKEN_FILE=$1
ACL_AGENT_TOKEN=$(cat $ACL_TOKEN_FILE)

MASTER_TOKEN=mastertoken
curl --request PUT --header "X-Consul-Token: $MASTER_TOKEN" --data  '{ "ID": "testtoken", "Name": "Agent Token", "Type": "client", "Rules": "node \"\" { policy = \"write\" } service \"\" { policy = \"write\" }" }' http://127.0.0.1:8500/v1/acl/create
curl --request PUT --header "X-Consul-Token: $MASTER_TOKEN" --data "{\"Token\": \"$ACL_AGENT_TOKEN\"}" http://127.0.0.1:8500/v1/agent/token/acl_agent_token
