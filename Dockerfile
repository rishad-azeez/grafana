FROM frolvlad/alpine-glibc:alpine-3.18

# Set the working directory inside the container
WORKDIR /app

# Copy the pre-built binaries and other necessary files
#COPY ./bin /app/bin
#COPY ./public /app/public
#COPY ./conf /app/conf

# Expose the port on which Grafana will run
EXPOSE 3000

# Run the Grafana server
CMD ["/app/bin/grafana-server", "server", "--homepath=/app"]
