// Copyright 2023 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package com.google.examples.xds.greeter.interceptors;

import com.google.examples.xds.greeter.logging.MessagePrinter;
import com.google.rpc.ErrorInfo;
import io.grpc.ForwardingServerCall.SimpleForwardingServerCall;
import io.grpc.ForwardingServerCallListener.SimpleForwardingServerCallListener;
import io.grpc.Metadata;
import io.grpc.ServerCall;
import io.grpc.ServerCall.Listener;
import io.grpc.ServerCallHandler;
import io.grpc.ServerInterceptor;
import io.grpc.Status;
import io.grpc.examples.helloworld.HelloReply;
import io.grpc.examples.helloworld.HelloRequest;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

/** Logs requests received and responses sent by a server. */
public class LoggingServerInterceptor implements ServerInterceptor {

  private static final Logger LOG = LoggerFactory.getLogger(LoggingServerInterceptor.class);

  private final MessagePrinter messagePrinter;

  /** Create a server interceptor configured to log HelloRequest and HelloReply messages as JSON. */
  public LoggingServerInterceptor() {
    this.messagePrinter =
        new MessagePrinter(
            HelloRequest.getDescriptor(), HelloReply.getDescriptor(), ErrorInfo.getDescriptor());
  }

  @Override
  public <ReqT, RespT> Listener<ReqT> interceptCall(
      ServerCall<ReqT, RespT> call, Metadata requestHeaders, ServerCallHandler<ReqT, RespT> next) {
    return new SimpleForwardingServerCallListener<>(
        next.startCall(
            new SimpleForwardingServerCall<ReqT, RespT>(call) {
              @Override
              public void sendMessage(RespT message) {
                if (LOG.isDebugEnabled()) {
                  LOG.debug(
                      "Sending response {}:\n{}",
                      message.getClass().getCanonicalName(),
                      messagePrinter.print(message));
                }
                super.sendMessage(message);
              }

              @Override
              public void close(Status status, Metadata trailers) {
                if (!status.isOk()) {
                  LOG.error(
                      "RPC status code: {} description: {}",
                      status.getCode(),
                      status.getDescription(),
                      status.asException(trailers));
                } else {
                  LOG.debug(
                      "RPC status code: {} description: {}",
                      status.getCode(),
                      status.getDescription());
                }
                super.close(status, trailers);
              }
            },
            requestHeaders)) {
      @Override
      public void onMessage(ReqT message) {
        if (LOG.isDebugEnabled()) {
          LOG.debug(
              "Received request {}:\n{}",
              message.getClass().getCanonicalName(),
              messagePrinter.print(message));
        }
        super.onMessage(message);
      }
    };
  }
}
