-- Post-Migration Security Setup
-- This script should be run AFTER the main migration with proper credentials

-- Set password for shard_1 user mapping
-- Replace 'YOUR_SECURE_PASSWORD_1' with actual password from secure configuration
ALTER USER MAPPING FOR CURRENT_USER 
    SERVER shard_1 
    OPTIONS (ADD password 'YOUR_SECURE_PASSWORD_1');

-- Set password for shard_2 user mapping  
-- Replace 'YOUR_SECURE_PASSWORD_2' with actual password from secure configuration
ALTER USER MAPPING FOR CURRENT_USER 
    SERVER shard_2 
    OPTIONS (ADD password 'YOUR_SECURE_PASSWORD_2');

-- Example of setting passwords from environment variables (run via psql):
-- \set shard1_password `echo $DCE_SHARD1_PASSWORD`
-- \set shard2_password `echo $DCE_SHARD2_PASSWORD`
-- ALTER USER MAPPING FOR CURRENT_USER SERVER shard_1 OPTIONS (ADD password :'shard1_password');
-- ALTER USER MAPPING FOR CURRENT_USER SERVER shard_2 OPTIONS (ADD password :'shard2_password');

-- SECURITY NOTES:
-- 1. Never commit actual passwords to version control
-- 2. Use environment variables or secure vaults (e.g., HashiCorp Vault, AWS Secrets Manager)
-- 3. Rotate passwords regularly
-- 4. Use different passwords for each shard
-- 5. Ensure passwords meet complexity requirements (min 16 chars, mixed case, numbers, symbols)