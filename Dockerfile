FROM alpine:3.22
WORKDIR /app

RUN addgroup -S appgroup && adduser -S appuser -G appgroup

COPY build/paye-ton-kawa--customers /app/paye-ton-kawa--customers

RUN chown appuser:appgroup /app/paye-ton-kawa--customers && chmod +x /app/paye-ton-kawa--customers

EXPOSE 8081

USER appuser

ENTRYPOINT ["/app/paye-ton-kawa--customers"]
