#! /bin/bash

jwtd() {
    read foo;
    jq -R 'split(".") |.[0:2] | map(gsub("-"; "+") | gsub("_"; "/") | gsub("%3D"; "=") | @base64d) | map(fromjson)' <<< $foo; 
}

function idp_mock_login() {
    IDP_URL=http://127.0.0.1:5550/oidc/

    TOKENS=`curl -d scope="openid profile email" -d grant_type=client_credentials "${IDP_URL}token"`
    ID_TOKEN=`echo $TOKENS | jq -r .id_token`

    echo $ID_TOKEN
}
