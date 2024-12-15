import unittest
import subprocess
import time
from client import RaftClient

node = "http://127.0.0.1:8000"

class T(unittest.TestCase):

    def setUp(self):
        self.process = subprocess.Popen(["../server/main", "8000", "8000", "500"], stdout=subprocess.DEVNULL, stderr=subprocess.PIPE)

    def tearDown(self):
        self.process.terminate()
        self.process.wait()

    def test_follower(self):
        time.sleep(0.05)
        client = RaftClient(node)
        role = client.current_role()
        print(role)
        assert role == "follower"

    def test_leader(self):
        time.sleep(1)
        client = RaftClient(node)
        role = client.current_role()
        assert role == "leader"
        
        code, value = client.read("key1")
        assert code == 404
        assert value == ""
        
        assert client.update("key1", "value1") == 200
        _, value = client.read("key1")
        assert value == "value1"
        assert client.update("key1", "value2") == 200
        _, value = client.read("key1")
        assert value == "value2"

        assert client.update("key2", "value10") == 200
        _, value = client.read("key1")
        assert value == "value2"
        _, value = client.read("key2")
        assert value == "value10"

        assert client.delete("key1") == 200
        code, value = client.read("key1")
        assert code == 404
        assert value == ""
        _, value = client.read("key2")
        assert value == "value10"

    def test_stop_recover(self):
        time.sleep(1)
        client = RaftClient(node)

        assert client.update("key1", "value1") == 200
        _, value = client.read("key1")
        assert value == "value1"

        client.stop()
        assert client.read("key1") == (403, "")
        assert client.update("key1", "value1") == 403

        client.recover()
        assert client.read("key1") == (200, "value1")


if __name__ == "__main__":
    unittest.main()
