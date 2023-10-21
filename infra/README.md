

This is configured for AWS EC2 Ubuntu instances, not for Amazon Linux. So the username is set to 'ubuntu' rather than 'ec2-user' throughout.


1. inventory.ini: change your user and key to match your EC2 instance's values.
2. gocsv.service.js: change values under [Service] as needed.
3. deploy.yml: Fill in password where you find "your_secure_password"



Deploy with HashiCorp Vault:

infra/ $ ansible-playbook -i inventory.ini deploy.yml --tags "vault"

Deploy without Vault (fall back to environment variables):

infra/ $ ansible-playbook -i inventory.ini deploy.yml




Copy inventory.ini_sample to inventory.ini and insert your own values.