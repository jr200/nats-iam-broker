#!/bin/bash

if [ $# -ne 1 ]; then
    echo "Usage: $0 <secrets-directory>"
    exit 1
fi

SECRETS_DIR=$1

# Check if directory exists
if [ ! -d "$SECRETS_DIR" ]; then
    echo "Error: Directory $SECRETS_DIR not found"
    exit 1
fi

# Create the secret manifest header
cat << EOF > nats-secrets.yaml
apiVersion: v1
kind: Secret
metadata:
  name: nats-secrets
type: Opaque
stringData:
EOF

# Process each file in the provided directory
for file in "$SECRETS_DIR"/*; do
    if [ -f "$file" ]; then
        # Get just the filename without the directory
        filename=$(basename "$file")
        
        # Add the file content to the secret
        echo "  $filename: |" >> nats-secrets.yaml
        # Indent the content by 4 spaces and add it to the yaml
        sed 's/^/    /' "$file" >> nats-secrets.yaml
        # Add a newline after each file
        echo "" >> nats-secrets.yaml
    fi
done

echo "Secret manifest has been created in nats-secrets.yaml"
echo "You can apply it using: kubectl apply -f nats-secrets.yaml"