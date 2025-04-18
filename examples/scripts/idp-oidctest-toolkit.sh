#! /bin/bash

jwtd() {
    read foo;
    jq -R 'split(".") |.[0:2] | map(gsub("-"; "+") | gsub("_"; "/") | gsub("%3D"; "=") | @base64d) | map(fromjson)' <<< $foo; 
}

function idp_oidctest_login_public() {
    local username=$1
    local password=$2

    IDP_URL=https://oidctest.wsweet.org/oauth2/

    SESSION_ID=$(curl -X POST -s -d "user=${username}" -d "password=${password}" -H 'Accept: application/json' "${IDP_URL}" | jq -r '.id')
    # echo $SESSION_ID

    LOCATION_HEADER=$(curl -s -D - -o /dev/null -b "lemonldap=${SESSION_ID}" "${IDP_URL}authorize?response_type=code&client_id=public&scope=openid+profile+email&redirect_uri=http://localhost" | grep -i '^location')
    # echo $LOCATION_HEADER

    CODE=$(echo "${LOCATION_HEADER}" | sed -n 's/.*code=\([^&]*\).*/\1/p')
    # echo $CODE

    TOKENS=$(curl -s -X POST -d "grant_type=authorization_code" -d 'redirect_uri=http://localhost' -d "code=${CODE}" -u public:public "${IDP_URL}token")
    ID_TOKEN=$(echo "$TOKENS" | jq -r .id_token)

    echo "$ID_TOKEN"
}

function idp_oidctest_login_private() {
    local username=$1
    local password=$2

    IDP_URL=https://oidctest.wsweet.org/oauth2/

    SESSION_ID=$(curl -X POST -s -d "user=${username}" -d "password=${password}" -H 'Accept: application/json' "${IDP_URL}" | jq -r '.id')
    # echo $SESSION_ID

    LOCATION_HEADER=$(curl -s -D - -o /dev/null -b "lemonldap=${SESSION_ID}" "${IDP_URL}authorize?response_type=code&client_id=private&scope=openid+profile+email&redirect_uri=http://localhost" | grep -i '^location')
    # echo $LOCATION_HEADER

    CODE=$(echo "${LOCATION_HEADER}" | sed -n 's/.*code=\([^&]*\).*/\1/p')
    # echo $CODE

    TOKENS=$(curl -s -X POST -d "grant_type=authorization_code" -d 'redirect_uri=http://localhost' -d "code=${CODE}" -u private:tardis "${IDP_URL}token")

    echo "$ID_TOKEN"
}

# curl -s -D - -o /dev/null -b lemonldap=${SESSION_ID} 'https://oidctest.wsweet.org/oauth2/logout?confirm=1'

