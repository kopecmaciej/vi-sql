-- Create extensions and types.
create extension if not exists hstore;

create database gaussdbgo_test DBCOMPATIBILITY 'pg';

-- Create users for different types of connections and authentication.
create user gaussdbgo_ssl with SYSADMIN PASSWORD '{{OPENGAUSS_PASSWORD}}';
create user gaussdbgo_sslcert with SYSADMIN PASSWORD '{{OPENGAUSS_PASSWORD}}';
create user gaussdbgo_md5 with SYSADMIN PASSWORD '{{OPENGAUSS_PASSWORD}}';
create user gaussdbgo_pw with SYSADMIN PASSWORD '{{OPENGAUSS_PASSWORD}}';
create user gaussdbgo_scram with SYSADMIN PASSWORD '{{OPENGAUSS_PASSWORD}}';

-- The tricky test user, below, has to actually exist so that it can be used in a test
-- of aclitem formatting. It turns out aclitems cannot contain non-existing users/roles.
-- todo ERROR:  Invalid name:  tricky, ' } " \\ test user .
--create user " tricky, ' } "" \\ test user " superuser password 'secret';
