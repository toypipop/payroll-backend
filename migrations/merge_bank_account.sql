-- Migration: Merge employee_bank_account into employees table
-- Date: 2026-04-07
-- Description: Move bank fields from employee_bank_account to employees table

-- Step 1: Add bank columns to employees table
ALTER TABLE employees 
ADD COLUMN bank_id BIGINT UNSIGNED NULL,
ADD COLUMN bank_name VARCHAR(100) NULL,
ADD COLUMN account_name VARCHAR(200) NULL,
ADD COLUMN account_no VARCHAR(50) NULL;

-- Step 2: Copy data from employee_bank_account to employees
UPDATE employees e
INNER JOIN employee_bank_account eb ON e.id = eb.employees_id
SET 
    e.bank_id = eb.bank_id,
    e.bank_name = eb.bank_name,
    e.account_name = eb.account_name,
    e.account_no = eb.account_no;

-- Step 3: Drop the employee_bank_account table (after verifying data migrated)
-- DROP TABLE IF EXISTS employee_bank_account;

-- Note: Run step 3 only after verifying the data has been migrated correctly
-- You can verify with: SELECT id, employees_code, bank_id, account_name, account_no FROM employees WHERE bank_id IS NOT NULL;
