from flask import Flask, jsonify, request
from pymongo import MongoClient
from bson import ObjectId
import os

app = Flask(__name__)

# Connect to MongoDB using the retrieved URI
client = MongoClient(os.environ.get('MONGO_URI'))
db = client['mongodb']
collection = db['mongodb']

# Get all products
@app.route('/products', methods=['GET'])
def get_products():
    products = db.products.find()
    result = []
    for product in products:
        result.append({
            'id': str(product['_id']),
            'name': product['name'],
            'description': product['description']
        })
    return jsonify(result)

# Get a single product
@app.route('/products/<product_id>', methods=['GET'])
def get_product(product_id):
    product = db.products.find_one({'_id': ObjectId(product_id)})
    if product:
        return jsonify({
            'id': str(product['_id']),
            'name': product['name'],
            'description': product['description']
        })
    else:
        return jsonify({'message': 'Product not found.'}), 404

# Add a new product
@app.route('/products', methods=['POST'])
def add_product():
    product = {
        'name': request.json['name'],
        'description': request.json['description']
    }
    result = db.products.insert_one(product)
    return jsonify({'message': 'Product added successfully', 'id': str(result.inserted_id)})

# Update a product
@app.route('/products/<product_id>', methods=['PUT'])
def update_product(product_id):
    updated_product = {
        'name': request.json['name'],
        'description': request.json['description']
    }
    db.products.update_one({'_id': ObjectId(product_id)}, {'$set': updated_product})
    return jsonify({'message': 'Product updated successfully'})

# Delete a product
@app.route('/products/<product_id>', methods=['DELETE'])
def delete_product(product_id):
    db.products.delete_one({'_id': ObjectId(product_id)})
    return jsonify({'message': 'Product deleted successfully'})

if __name__ == '__main__':
    app.run(host='0.0.0.0', port=8085, debug=True)
