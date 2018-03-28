ACL_TOKEN_FILE=$1
ACL_AGENT_TOKEN=$(cat $ACL_TOKEN_FILE)

apk update
apk add jq

MASTER_TOKEN=mastertoken
curl --request PUT --header "X-Consul-Token: $MASTER_TOKEN" --data  '{ "ID": "testtoken", "Name": "Agent Token", "Type": "client", "Rules": "node \"\" { policy = \"write\" } service \"\" { policy = \"write\" }" }' http://127.0.0.1:8500/v1/acl/create | jq '.ID'
curl --request PUT --header "X-Consul-Token: $MASTER_TOKEN" --data "{\"Token\": $AGENT_ACL_TOKEN}" http://127.0.0.1:8500/v1/agent/token/acl_agent_token
