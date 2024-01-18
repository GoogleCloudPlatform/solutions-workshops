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

package com.google.examples.xds.greeter;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.junit.jupiter.api.Assertions.fail;

import com.google.examples.xds.greeter.service.GreeterClient;
import com.google.examples.xds.greeter.service.GreeterException;
import com.google.examples.xds.greeter.service.GreeterIntermediary;
import io.grpc.Status.Code;
import io.grpc.StatusRuntimeException;
import io.grpc.examples.helloworld.HelloReply;
import io.grpc.examples.helloworld.HelloRequest;
import io.grpc.stub.StreamObserver;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.TimeUnit;
import org.junit.jupiter.api.Test;

/** Tests for GreeterIntermediary. */
public class GreeterIntermediaryTest {

  /** Test error handling. */
  @Test
  public void sayHelloError() throws Exception {
    final CountDownLatch latch = new CountDownLatch(1);
    var greeterIntermediary =
        new GreeterIntermediary(
            "name",
            new GreeterClient("localhost", false) {
              @Override
              public String sayHello(String name) {
                throw new GreeterException("Greeter exception");
              }
            });

    greeterIntermediary.sayHello(
        HelloRequest.newBuilder().setName("name").build(),
        new StreamObserver<>() {
          @Override
          public void onNext(HelloReply value) {
            fail();
          }

          @Override
          public void onError(Throwable t) {
            assertTrue(StatusRuntimeException.class.isAssignableFrom(t.getClass()));
            var sre = (StatusRuntimeException) t;
            assertEquals(Code.INTERNAL.value(), sre.getStatus().getCode().value());
            latch.countDown();
          }

          @Override
          public void onCompleted() {
            fail();
          }
        });
    assertTrue(latch.await(3, TimeUnit.SECONDS));
  }
}
