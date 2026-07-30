# GoMockAPI
Mock APIs with ease 

GoMockAPI is a flexible API mocking tool built in Go that allows you to mock REST APIs for testing purposes. Users can configure multiple endpoints with various HTTP response codes and payloads. Additionally, responses can be dynamically triggered based on user-defined conditions, allowing for more complex and realistic test scenarios.


# Features
- Multiple configurable endpoints: Easily set up different routes with custom responses.
- Custom HTTP responses: Specify different HTTP codes (200, 404, 500, etc.) and payloads for each endpoint.
- Conditional responses: Trigger different responses based on conditions like request headers, query parameters, or request body content.
- Simulated delays: Add delays to simulate network latency and test client-side timeouts.
- Dynamic response generation: Use templates to generate dynamic responses using request data.
- Easy setup: Configure endpoints and responses using simple interface. Or upload postman json files.
- Lightweight: Minimal dependencies and simple to run.
- No external dependencies
- Inbuilt database.
 
# Docker Image

- docker pull onlysumitg/gomockapi


# Installation
1. Make sure you have Go installed on your system. If not, download it from https://go.dev/dl/
2. Install gomockapi by running the following command:
    1. go install github.com/yourusername/gomockapi@latest
3. Clone the repository if you want to modify the code:
    ```    
    git clone https://github.com/yourusername/gomockapi.git
    ```
4. To start app
    ```
    1. cd gomockapi
    2. go run ./cmd/web
    ```


# Default credentials
```
user: admin2@example.com
pwd: adminpass
```

# Env variables
```
PORT=4081
DOMAIN=myapi.com
ALLOWEDORIGINS=https://*,http://*
DEBUG=false
HTTPS=false
USELETSENCRYPT=false

REQUESTS_PER_HOUR_BY_IP=1000
REQUESTS_PER_HOUR_BY_USER=1000

SMTP_HOST=smtp.examplecom
SMTP_PORT=587
SMTP_USERNAME=myemail@example.com
SMTP_PASSWORD=mypassword
```




 


