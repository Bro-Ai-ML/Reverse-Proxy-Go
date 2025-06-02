-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Customers table
CREATE TABLE IF NOT EXISTS customers (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL UNIQUE,
    email VARCHAR(255) NOT NULL,
    name VARCHAR(255),
    phone VARCHAR(50),
    address JSONB,
    tax_id VARCHAR(100),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

-- Payment methods table
CREATE TYPE payment_method_type AS ENUM ('card', 'bank_account', 'paypal', 'other');

CREATE TABLE IF NOT EXISTS payment_methods (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    customer_id UUID NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
    type payment_method_type NOT NULL,
    payment_provider_id VARCHAR(255) NOT NULL,
    is_default BOOLEAN NOT NULL DEFAULT FALSE,
    details JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    UNIQUE(customer_id, payment_provider_id)
);

-- Products table
CREATE TABLE IF NOT EXISTS products (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    description TEXT,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

-- Prices table
CREATE TYPE price_type AS ENUM ('one_time', 'recurring');
CREATE TYPE price_interval AS ENUM ('day', 'week', 'month', 'year');

CREATE TABLE IF NOT EXISTS prices (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    product_id UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    currency VARCHAR(3) NOT NULL,
    unit_amount BIGINT NOT NULL,
    type price_type NOT NULL,
    interval price_interval,
    interval_count INTEGER,
    trial_period_days INTEGER,
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    UNIQUE(product_id, type, interval, interval_count, unit_amount, currency)
);

-- Subscriptions table
CREATE TYPE subscription_status AS ENUM ('incomplete', 'incomplete_expired', 'trialing', 'active', 'past_due', 'canceled', 'unpaid');

CREATE TABLE IF NOT EXISTS subscriptions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    customer_id UUID NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
    price_id UUID NOT NULL REFERENCES prices(id) ON DELETE RESTRICT,
    status subscription_status NOT NULL,
    current_period_start TIMESTAMPTZ NOT NULL,
    current_period_end TIMESTAMPTZ NOT NULL,
    cancel_at TIMESTAMPTZ,
    canceled_at TIMESTAMPTZ,
    ended_at TIMESTAMPTZ,
    trial_start TIMESTAMPTZ,
    trial_end TIMESTAMPTZ,
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

-- Invoices table
CREATE TYPE invoice_status AS ENUM ('draft', 'open', 'paid', 'void', 'uncollectible');

CREATE TABLE IF NOT EXISTS invoices (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    customer_id UUID NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
    subscription_id UUID REFERENCES subscriptions(id) ON DELETE SET NULL,
    status invoice_status NOT NULL,
    number VARCHAR(50) NOT NULL,
    currency VARCHAR(3) NOT NULL,
    subtotal BIGINT NOT NULL,
    tax BIGINT NOT NULL DEFAULT 0,
    total BIGINT NOT NULL,
    amount_due BIGINT NOT NULL,
    amount_paid BIGINT NOT NULL DEFAULT 0,
    amount_remaining BIGINT NOT NULL,
    due_date TIMESTAMPTZ,
    paid_at TIMESTAMPTZ,
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    UNIQUE(number)
);

-- Invoice line items
CREATE TABLE IF NOT EXISTS invoice_items (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    invoice_id UUID NOT NULL REFERENCES invoices(id) ON DELETE CASCADE,
    price_id UUID REFERENCES prices(id) ON DELETE SET NULL,
    subscription_id UUID REFERENCES subscriptions(id) ON DELETE SET NULL,
    amount BIGINT NOT NULL,
    currency VARCHAR(3) NOT NULL,
    quantity INTEGER NOT NULL DEFAULT 1,
    description TEXT,
    period_start TIMESTAMPTZ,
    period_end TIMESTAMPTZ,
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

-- Payments table
CREATE TYPE payment_status AS ENUM ('pending', 'requires_payment_method', 'requires_confirmation', 'requires_action', 'processing', 'requires_capture', 'canceled', 'succeeded', 'failed');

CREATE TABLE IF NOT EXISTS payments (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    customer_id UUID NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
    invoice_id UUID REFERENCES invoices(id) ON DELETE SET NULL,
    payment_method_id UUID REFERENCES payment_methods(id) ON DELETE SET NULL,
    amount BIGINT NOT NULL,
    currency VARCHAR(3) NOT NULL,
    status payment_status NOT NULL,
    payment_intent_id VARCHAR(255),
    last_payment_error TEXT,
    receipt_url TEXT,
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

-- Usage records
CREATE TABLE IF NOT EXISTS usage_records (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    subscription_item_id UUID NOT NULL,
    customer_id UUID NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
    quantity INTEGER NOT NULL,
    timestamp TIMESTAMPTZ NOT NULL,
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

-- Webhook events
CREATE TABLE IF NOT EXISTS webhook_events (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    event_id VARCHAR(255) NOT NULL UNIQUE,
    event_type VARCHAR(255) NOT NULL,
    api_version VARCHAR(50),
    payload JSONB NOT NULL,
    processed BOOLEAN NOT NULL DEFAULT FALSE,
    error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Create indexes
CREATE INDEX IF NOT EXISTS idx_customers_user_id ON customers(user_id);
CREATE INDEX IF NOT EXISTS idx_payment_methods_customer_id ON payment_methods(customer_id);
CREATE INDEX IF NOT EXISTS idx_subscriptions_customer_id ON subscriptions(customer_id);
CREATE INDEX IF NOT EXISTS idx_subscriptions_status ON subscriptions(status);
CREATE INDEX IF NOT EXISTS idx_invoices_customer_id ON invoices(customer_id);
CREATE INDEX IF NOT EXISTS idx_invoices_status ON invoices(status);
CREATE INDEX IF NOT EXISTS idx_invoice_items_invoice_id ON invoice_items(invoice_id);
CREATE INDEX IF NOT EXISTS idx_payments_customer_id ON payments(customer_id);
CREATE INDEX IF NOT EXISTS idx_payments_status ON payments(status);
CREATE INDEX IF NOT EXISTS idx_usage_records_customer_id ON usage_records(customer_id);
CREATE INDEX IF NOT EXISTS idx_usage_records_timestamp ON usage_records(timestamp);

-- Create function to update updated_at column
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Create triggers to update updated_at
CREATE TRIGGER update_customers_updated_at
BEFORE UPDATE ON customers
FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_payment_methods_updated_at
BEFORE UPDATE ON payment_methods
FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_products_updated_at
BEFORE UPDATE ON products
FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_prices_updated_at
BEFORE UPDATE ON prices
FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_subscriptions_updated_at
BEFORE UPDATE ON subscriptions
FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_invoices_updated_at
BEFORE UPDATE ON invoices
FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_invoice_items_updated_at
BEFORE UPDATE ON invoice_items
FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_payments_updated_at
BEFORE UPDATE ON payments
FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_usage_records_updated_at
BEFORE UPDATE ON usage_records
FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_webhook_events_updated_at
BEFORE UPDATE ON webhook_events
FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Create function to generate invoice numbers
CREATE OR REPLACE FUNCTION generate_invoice_number()
RETURNS TRIGGER AS $$
DECLARE
    prefix TEXT := 'INV';
    year_part TEXT := TO_CHAR(NOW(), 'YY');
    month_part TEXT := TO_CHAR(NOW(), 'MM');
    seq_num INTEGER;
    new_number TEXT;
BEGIN
    -- Get the next sequence value for this month
    SELECT COALESCE(MAX(SUBSTRING(number, 10)::INTEGER), 0) + 1
    INTO seq_num
    FROM invoices
    WHERE SUBSTRING(number, 1, 8) = prefix || year_part || month_part;
    
    -- Format the invoice number (e.g., INV2301001)
    new_number := prefix || year_part || month_part || LPAD(seq_num::TEXT, 4, '0');
    
    NEW.number := new_number;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Create trigger to generate invoice numbers
CREATE TRIGGER set_invoice_number
BEFORE INSERT ON invoices
FOR EACH ROW
WHEN (NEW.number IS NULL)
EXECUTE FUNCTION generate_invoice_number();
