import unittest
import subprocess
import time
from client import RaftClient

node = "http://127.0.0.1:8000"

class T(unittest.TestCase):

    # @classmethod
    # def setUpClass(cls):
    #     subprocess.Popen(["../server/main", "8000", "8000", "500"])
    #     time.sleep(1)

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
        
        client.update("key1", "value1")
        _, value = client.read("key1")
        assert value == "value1"
        client.update("key1", "value2")
        _, value = client.read("key1")
        assert value == "value2"

        client.update("key2", "value10")
        _, value = client.read("key1")
        assert value == "value2"
        _, value = client.read("key2")
        assert value == "value10"

        client.delete("key1")
        code, value = client.read("key1")
        assert code == 404
        assert value == ""
        _, value = client.read("key2")
        assert value == "value10"


if __name__ == "__main__":
    unittest.main()
