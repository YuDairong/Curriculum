from flask import Flask, request, jsonify
import pika
import asyncio
# import aiohttp
import os

app = Flask(__name__)

# Connect to RabbitMQ
connection = pika.BlockingConnection(pika.ConnectionParameters(os.environ.get('SHIPPING_SERVICE_HOST')))
channel = connection.channel()

# Declare the queue
channel.queue_declare(queue='shipping_queue')

async def receive_order_messages():

    # Bind the queue to the exchange
    channel.queue_bind(exchange='order_exchange', queue='shipping_queue', routing_key='')

    # Define the callback function for receiving messages
    def callback(ch, method, properties, body):
        create_shipping()

    # Start consuming messages from the queue
    channel.basic_consume(queue='shipping_queue', on_message_callback=callback, auto_ack=True)

    print('Waiting for order messages. To exit, press CTRL+C')

    # Start consuming messages indefinitely
    channel.start_consuming()


async def create_shipping():
    print('Received order messages.')
    # order_id = request.json.get('order_id')
    # address = request.json.get('address')
    
    # # Process the shipping request asynchronously
    # asyncio.create_task(process_shipping(order_id, address))

    # shipping_message = f"Shipping created for OrderID: {order_id}"
    # channel.basic_publish(exchange='', routing_key='shipping_queue', body=shipping_message)

    # connection.close()

    # return jsonify({'message': 'Shipping created successfully'})

async def process_shipping(order_id, address):
    # Simulate some processing time
    await asyncio.sleep(3)
    
    # Make a POST request to the order service to update the order status
    async with aiohttp.ClientSession() as session:
        async with session.post('http://localhost:8082/orders', json={'order_id': order_id, 'status': 'shipped'}) as response:
            if response.status == 200:
                print(f"Shipping completed for Order {order_id}. Status updated in order service.")
            else:
                print(f"Failed to update order status for Order {order_id}.")

if __name__ == '__main__':
    app.run(debug=True, host='0.0.0.0', port=8001)
