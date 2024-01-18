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

package com.google.examples.xds.greeter.service;

import io.grpc.examples.helloworld.GreeterGrpc.GreeterImplBase;
import io.grpc.examples.helloworld.HelloReply;
import io.grpc.examples.helloworld.HelloRequest;
import io.grpc.stub.StreamObserver;
import org.jetbrains.annotations.NotNull;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

/**
 * Greeter implementation which replies with its name.
 *
 * @see GreeterIntermediary for the other implementation.
 */
public class GreeterLeaf extends GreeterImplBase {
  private static final Logger LOG = LoggerFactory.getLogger(GreeterLeaf.class);

  private final String name;

  public GreeterLeaf(@NotNull String name) {
    this.name = name;
  }

  @Override
  public void sayHello(HelloRequest request, StreamObserver<HelloReply> responseObserver) {
    String name = request.getName();
    LOG.info("Received request to greet {}, returning greeting.", name);
    var reply = HelloReply.newBuilder().setMessage("Hello " + name + ", from " + this.name).build();
    responseObserver.onNext(reply);
    responseObserver.onCompleted();
  }
}
