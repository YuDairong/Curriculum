const express = require('express');
const app = express();

// Middleware to parse JSON request bodies
app.use(express.json());

const { Pool } = require('pg');

// Retrieve the PostgreSQL URI from the environment variable
const postgresURI = process.env.POSTGRES_URI;
const pool = new Pool({ connectionString: postgresURI });

// Retry configuration
const retryOperation = async (operation, maxAttempts, delay) => {
  let attempts = 0;
  while (attempts < maxAttempts) {
    try {
      await operation();
      return;
    } catch (error) {
      attempts++;
      console.error(`Attempt ${attempts} failed with error: ${error.message}`);
      await new Promise(resolve => setTimeout(resolve, delay));
    }
  }
  console.error(`Operation failed after ${maxAttempts} attempts.`);
};

async function checkAndCreateUsersTable() {
  const maxAttempts = 5;
  const delay = 1000;

  const operation = async () => {
    const result = await pool.query(`
      SELECT EXISTS (
        SELECT 1
        FROM information_schema.tables
        WHERE table_name = 'users'
      )
    `);
    const tableExists = result.rows[0].exists;

    if (!tableExists) {
      await pool.query(`
        CREATE TABLE users (
          id SERIAL PRIMARY KEY,
          name VARCHAR(255),
          email VARCHAR(255)
        )
      `);
      console.log('Users table created');
    } else {
      console.log('Users table already exists');
    }
  };

  await retryOperation(operation, maxAttempts, delay);
}

checkAndCreateUsersTable();

// Get all users
app.get('/users', async (req, res) => {
  try {
    const { rows } = await pool.query('SELECT * FROM users');
    res.json(rows);
  } catch (error) {
    console.error('Error retrieving users:', error);
    res.status(500).json({ error: 'Internal server error' });
  }
});

// Get a specific user by ID
app.get('/users/:id', async (req, res) => {
  const { id } = req.params;
  try {
    const { rows } = await pool.query('SELECT * FROM users WHERE id = $1', [id]);
    if (rows.length === 0) {
      res.status(404).json({ error: 'User not found' });
    } else {
      res.json(rows[0]);
    }
  } catch (error) {
    console.error('Error retrieving user:', error);
    res.status(500).json({ error: 'Internal server error' });
  }
});

// Create a new user
app.post('/users', async (req, res) => {
  const { name, email } = req.body;
  try {
    const { rows } = await pool.query('INSERT INTO users (name, email) VALUES ($1, $2) RETURNING *', [name, email]);
    res.status(201).json(rows[0]);
  } catch (error) {
    console.error('Error creating user:', error);
    res.status(500).json({ error: 'Internal server error' });
  }
});

// Update an existing user
app.put('/users/:id', async (req, res) => {
  const { id } = req.params;
  const { name, email } = req.body;
  try {
    const { rows } = await pool.query('UPDATE users SET name = $1, email = $2 WHERE id = $3 RETURNING *', [name, email, id]);
    if (rows.length === 0) {
      res.status(404).json({ error: 'User not found' });
    } else {
      res.json(rows[0]);
    }
  } catch (error) {
    console.error('Error updating user:', error);
    res.status(500).json({ error: 'Internal server error' });
  }
});

// Delete a user
app.delete('/users/:id', async (req, res) => {
  const { id } = req.params;
  try {
    const { rows } = await pool.query('DELETE FROM users WHERE id = $1 RETURNING *', [id]);
    if (rows.length === 0) {
      res.status(404).json({ error: 'User not found' });
    } else {
      res.status(200).json({ message: 'User deleted successfully' });
    }
  } catch (error) {
    console.error('Error deleting user:', error);
    res.status(500).json({ error: 'Internal server error' });
  }
});

// Delete all users
app.delete('/users', async (req, res) => {
  try {
    await pool.query('TRUNCATE TABLE users'); // Delete all rows and reset auto-increment counter
    await pool.query('ALTER SEQUENCE users_id_seq RESTART WITH 1');
    res.status(200).json({ message: 'All users deleted successfully' });
  } catch (error) {
    console.error('Error deleting users:', error);
    res.status(500).json({ error: 'Internal server error' });
  }
});

app.listen(8086, () => {
  console.log('Server is running on port 8086');
});
