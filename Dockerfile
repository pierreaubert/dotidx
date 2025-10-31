FROM dotidx:base

# Avoid interactive prompts during build
ENV USER=dotuser

# Create dotidx user and directories
RUN id -u ${USER} || /usr/sbin/useradd -m -s /bin/bash ${USER}

# Copy source code
COPY --chown=${USER}:${USER} . /src/dotidx
WORKDIR /src/dotidx

# Build the project
RUN make

# Initialize PostgreSQL
USER postgres
RUN /usr/lib/postgresql/16/bin/initdb -D /var/lib/postgresql/16/main

# Copy binaries to /dotidx/bin
RUN bin/dixmgr -conf ./conf/etc-docker-test.toml

# Expose ports
EXPOSE 8080
