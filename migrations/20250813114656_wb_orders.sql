-- +goose Up

CREATE SCHEMA IF NOT EXISTS wb;
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- важное: выполняем функцию единым стейтментом
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION wb.set_updated_at()
RETURNS trigger AS $$
BEGIN
  NEW.updated_at = now();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TABLE wb.orders (
    id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    order_uid          text NOT NULL UNIQUE,
    track_number       text NOT NULL,
    entry              text,
    locale             text,
    internal_signature text,
    customer_id        text,
    delivery_service   text,
    shardkey           text,
    sm_id              integer,
    date_created       timestamptz NOT NULL,
    oof_shard          text,
    payload            jsonb,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now()
);

CREATE TRIGGER trg_orders_updated
BEFORE UPDATE ON wb.orders
FOR EACH ROW EXECUTE FUNCTION wb.set_updated_at();

CREATE INDEX idx_orders_track_number ON wb.orders(track_number);
CREATE INDEX idx_orders_date_created ON wb.orders(date_created);
CREATE INDEX idx_orders_customer_id ON wb.orders(customer_id);

CREATE TABLE wb.delivery (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  order_id   uuid NOT NULL UNIQUE REFERENCES wb.orders(id) ON DELETE CASCADE,
  name       text,
  phone      text,
  zip        text,
  city       text,
  address    text,
  region     text,
  email      text,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TRIGGER trg_delivery_updated
BEFORE UPDATE ON wb.delivery
FOR EACH ROW EXECUTE FUNCTION wb.set_updated_at();

CREATE TABLE wb.payment (
  id                   uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  order_id             uuid NOT NULL UNIQUE REFERENCES wb.orders(id) ON DELETE CASCADE,
  transaction          text NOT NULL,
  request_id           text,
  currency             text NOT NULL,
  provider             text,
  amount_cents         integer NOT NULL,
  payment_dt           bigint NOT NULL,
  payment_at           timestamptz GENERATED ALWAYS AS (to_timestamp(payment_dt)) STORED,
  bank                 text,
  delivery_cost_cents  integer,
  goods_total_cents    integer,
  custom_fee_cents     integer,
  created_at           timestamptz NOT NULL DEFAULT now(),
  updated_at           timestamptz NOT NULL DEFAULT now()
);

CREATE TRIGGER trg_payment_updated
BEFORE UPDATE ON wb.payment
FOR EACH ROW EXECUTE FUNCTION wb.set_updated_at();

CREATE INDEX idx_payment_payment_at ON wb.payment(payment_at);

CREATE TABLE wb.items (
  id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  order_id           uuid NOT NULL REFERENCES wb.orders(id) ON DELETE CASCADE,
  chrt_id            bigint,
  track_number       text,
  price_cents        integer NOT NULL,
  rid                text,
  name               text,
  sale               integer,
  size               text,
  total_price_cents  integer NOT NULL,
  nm_id              bigint,
  brand              text,
  status             integer,
  created_at         timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_items_order_id ON wb.items(order_id);
CREATE INDEX idx_items_nm_id   ON wb.items(nm_id);
CREATE INDEX idx_items_status  ON wb.items(status);

-- +goose Down
DROP TABLE IF EXISTS wb.items;
DROP TABLE IF EXISTS wb.payment;
DROP TABLE IF EXISTS wb.delivery;
DROP TABLE IF EXISTS wb.orders;
DROP FUNCTION IF EXISTS wb.set_updated_at;
-- DROP SCHEMA IF EXISTS wb CASCADE;
