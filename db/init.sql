CREATE TABLE IF NOT EXISTS brands (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL UNIQUE,
    country VARCHAR(100) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO brands (name, country) VALUES
    ('Toyota', 'Japan'), ('Honda', 'Japan'), ('Ford', 'United States'),
    ('Chevrolet', 'United States'), ('Volkswagen', 'Germany'),
    ('BMW', 'Germany'), ('Mercedes-Benz', 'Germany'), ('Audi', 'Germany'),
    ('Volvo', 'Sweden'), ('Fiat', 'Italy'), ('Renault', 'France'),
    ('Peugeot', 'France'), ('Hyundai', 'South Korea'), ('Kia', 'South Korea'),
    ('Nissan', 'Japan'), ('Mazda', 'Japan'), ('Subaru', 'Japan'),
    ('Mitsubishi', 'Japan'), ('Jeep', 'United States'), ('Porsche', 'Germany'),
    ('Ferrari', 'Italy'), ('Lamborghini', 'Italy'), ('Tesla', 'United States'),
    ('Land Rover', 'United Kingdom'), ('Jaguar', 'United Kingdom')
ON CONFLICT (name) DO NOTHING;

-- The benchmark's default run uses 100 distinct rows.
INSERT INTO brands (name, country)
SELECT 'Benchmark Brand ' || LPAD(series::TEXT, 3, '0'), 'Benchmark Country'
FROM generate_series(1, 100) AS series
ON CONFLICT (name) DO NOTHING;
