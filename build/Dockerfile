# Backend build stage
FROM golang:1.25-bookworm AS backend
ARG VERSION
ARG BUILD_WEBUI=false
WORKDIR /src

# Install system dependencies first
RUN apt-get update && apt-get install -y \
    curl \
    autoconf \
    automake \
    libtool \
    pkg-config \
    git

# Clone and build libpostal (rarely changes)
RUN git clone https://github.com/openvenues/libpostal.git /src/libpostal
WORKDIR /src/libpostal
RUN ./bootstrap.sh && \
    ./configure && \
    make -j$(nproc) && \
    make install && \
    ldconfig

# Download libpostal data (this downloads ~2GB of language models)
RUN libpostal_data download all /usr/local/share/libpostal

# Copy go.mod and go.sum first to cache dependencies
COPY go.mod go.sum /src/
RUN go mod download

# Install fyne CLI for webui build (only if BUILD_WEBUI=true)
RUN if [ "$BUILD_WEBUI" = "true" ]; then \
        go install fyne.io/tools/cmd/fyne@latest; \
    fi

# Now copy the rest of the source code (frequently changes)
COPY . /src/
WORKDIR /src

# Build webui first (only if BUILD_WEBUI=true) - must happen before build-server
# so the wasm files are available for embedding
RUN if [ "$BUILD_WEBUI" = "true" ]; then \
        cd ./cmd/ui/ && fyne package --release --icon ./assets/icon.jpeg -os web --app-version "0.0.0" && cd -; \
    fi

# Build server binaries (will embed webui if it was built above)
RUN VERSION=${VERSION} GOTAGS="-tags libpostal" make build-server postal-server

# Final stage
FROM debian:trixie
LABEL maintainer="Moov <oss@moov.io>"

# Install runtime dependencies
RUN apt-get update && \
    apt-get install -y \
    tini \
    libssl3 \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Create necessary directories and copy libpostal files
RUN mkdir -p /usr/local/share/libpostal
COPY --from=backend /usr/local/lib/libpostal.so* /usr/local/lib/
COPY --from=backend /usr/local/lib/pkgconfig/libpostal.pc /usr/local/lib/pkgconfig/
COPY --from=backend /usr/local/share/libpostal/ /usr/local/share/libpostal/
RUN ldconfig

# Copy application files
COPY --from=backend /src/bin/server /bin/server
COPY --from=backend /src/bin/postal-server /bin/postal-server
COPY --from=backend /src/configs/config.all-lists.yml /etc/watchman/config.yml
ENV POSTAL_SERVER_BIN_PATH=/bin/postal-server

# Set environment variables
ENV LD_LIBRARY_PATH=/usr/local/lib
ENV LIBPOSTAL_DATA_DIR=/usr/local/share/libpostal
ENV APP_CONFIG_SECRETS=/etc/watchman/config.yml

EXPOSE 8084
EXPOSE 9094

ENTRYPOINT ["/usr/bin/tini", "--"]
CMD ["/bin/server"]
